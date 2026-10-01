package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/stt"
)

// CaptionTrackIDBase reserves canonical text IDs above node-added AV renditions.
const CaptionTrackIDBase uint32 = 100

// autoCaptionHold keeps recognized speech on screen after its last word
// until the next words replace it, so captions read without blinking off
// between agreed batches.
const autoCaptionHold = 3 * time.Second

type captionSession interface {
	captionPolicy() (captions.Policy, error)
	push(captions.Track, []captions.Cue) error
}

type captionMaster struct {
	ctx                    context.Context
	streamer               string
	sessionID              string
	cli                    *config.CLI
	engine                 stt.Engine
	hub                    *captions.Hub
	mu                     sync.Mutex
	current                captions.Policy
	policyReady            bool
	stopped                bool
	ingestSeen             bool
	recognitionUnavailable bool
	parsedUntil            uint64
	mediaFinished          bool
	arrival                time.Time
	mediaOrigin            time.Time
	covered                time.Time
	changed                chan struct{}
	closes                 map[uint64]time.Time
	gopTimes               map[uint64]time.Time
	signedUntil            uint64
	consumed               map[string]int64
	pending                map[string]muxl.TextCue
	tracks                 map[string]muxl.TextTrack
	next                   map[string]int64 // per track, the earliest start of its next cue
	nextID                 uint32
}

func newCaptionMaster(ctx context.Context, streamer string, cli *config.CLI, engine stt.Engine) *captionMaster {
	return &captionMaster{ctx: ctx, streamer: streamer, sessionID: uuid.NewString(), cli: cli, engine: engine, hub: captions.NewHub(0), current: captions.DefaultPolicy(), changed: make(chan struct{}), closes: make(map[uint64]time.Time), gopTimes: make(map[uint64]time.Time), consumed: make(map[string]int64), pending: make(map[string]muxl.TextCue), tracks: make(map[string]muxl.TextTrack), next: make(map[string]int64)}
}

func captionPolicyFromManifest(data []byte) captions.Policy {
	var manifest struct {
		Assertions []struct {
			Label string          `json:"label"`
			Data  json.RawMessage `json:"data"`
		} `json:"assertions"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return captions.DefaultPolicy()
	}
	for _, a := range manifest.Assertions {
		if a.Label == "place.stream.metadata.configuration" {
			var cfg placestream.MetadataConfiguration
			if json.Unmarshal(a.Data, &cfg) == nil {
				return captions.PolicyFromMetadata(&cfg)
			}
		}
	}
	return captions.DefaultPolicy()
}

func (m *captionMaster) setManifest(data []byte) {
	m.mu.Lock()
	m.current = captionPolicyFromManifest(data)
	m.policyReady = true
	m.signal()
	m.mu.Unlock()
}
func (m *captionMaster) policy() captions.Policy { m.mu.Lock(); defer m.mu.Unlock(); return m.current }
func (m *captionMaster) captionPolicy() (captions.Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.policyReady {
		return captions.Policy{}, fmt.Errorf("caption manifest not ready")
	}
	return m.current, nil
}

// waitPolicy prevents ingest from spending recognition budget on an unknown
// startup policy. The separate media queue keeps the signer free to fetch it.
func (m *captionMaster) waitPolicy() (captions.Policy, error) {
	m.mu.Lock()
	for !m.policyReady {
		if m.stopped {
			m.mu.Unlock()
			return captions.Policy{}, context.Canceled
		}
		changed := m.changed
		m.mu.Unlock()
		select {
		case <-m.ctx.Done():
			return captions.Policy{}, m.ctx.Err()
		case <-changed:
		}
		m.mu.Lock()
	}
	policy := m.current
	m.mu.Unlock()
	return policy, nil
}
func (m *captionMaster) signal() { close(m.changed); m.changed = make(chan struct{}) }
func (m *captionMaster) coverage(end time.Time) {
	m.mu.Lock()
	if end.After(m.covered) {
		m.covered = end
		m.signal()
	}
	m.mu.Unlock()
}
func (m *captionMaster) clockAt(media, at time.Time) {
	m.mu.Lock()
	if m.arrival.IsZero() {
		m.arrival = at
		m.mediaOrigin = time.UnixMilli(media.UnixMilli())
		m.signal()
	}
	m.mu.Unlock()
}

func (m *captionMaster) segmentTime(start uint64) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Clock readiness is independent of caption policy and recognition delay.
	// The signer queue can reach a GoP before the ingest parser sees its clock.
	for !m.mediaFinished && (m.arrival.IsZero() || m.parsedUntil < start) {
		changed := m.changed
		m.mu.Unlock()
		select {
		case <-m.ctx.Done():
			m.mu.Lock()
			return time.Time{}
		case <-changed:
		}
		m.mu.Lock()
	}
	prediction := m.arrival.Add(time.Duration(int64(start)-m.mediaOrigin.UnixMilli()) * time.Millisecond)
	if at, ok := m.gopTimes[start]; ok {
		prediction = at
	}
	for key := range m.gopTimes {
		if key <= start {
			delete(m.gopTimes, key)
		}
	}
	return prediction
}
func (m *captionMaster) closeGopAt(end uint64, at time.Time) {
	m.mu.Lock()
	prediction := m.arrival.Add(time.Duration(int64(end)-m.mediaOrigin.UnixMilli()) * time.Millisecond)
	if at.Sub(prediction).Abs() > time.Second {
		m.arrival = at
		m.mediaOrigin = time.UnixMilli(int64(end))
		prediction = at
	}
	m.gopTimes[end] = prediction
	if end > m.parsedUntil {
		m.parsedUntil = end
	}
	if end > m.signedUntil {
		if _, ok := m.closes[end]; !ok {
			m.closes[end] = at
		}
	}
	m.signal()
	m.mu.Unlock()
}
func (m *captionMaster) finishMedia() { m.mu.Lock(); m.mediaFinished = true; m.signal(); m.mu.Unlock() }
func (m *captionMaster) push(track captions.Track, cues []captions.Cue) error {
	m.mu.Lock()
	if m.arrival.IsZero() {
		m.mu.Unlock()
		return fmt.Errorf("caption media clock not ready")
	}
	arrival, origin := m.arrival, m.mediaOrigin
	m.ingestSeen = true
	m.signal()
	m.mu.Unlock()
	track.Origin = captions.OriginCanonical
	track.ID = "push-" + captions.TrackID(track.Origin, track.Source, track.Language)
	for _, cue := range cues {
		cue.Start = origin.Add(cue.Start.Sub(arrival))
		cue.End = origin.Add(cue.End.Sub(arrival))
		m.hub.Publish(m.streamer, track, cue)
	}
	return nil
}

// text holds only the signer, never appsink. It consumes immutable final cues
// from a private hub. A late final is carried into the next unsigned GoP.
func (m *captionMaster) text(ctx context.Context, req muxl.TextRequest) (*muxl.TextAttachment, error) {
	m.mu.Lock()
	closeAt, ok := m.closes[req.EndMs]
	if !ok {
		closeAt = time.Now()
	}
	for end := range m.closes {
		if end <= req.EndMs {
			delete(m.closes, end)
		}
	}
	delay := m.cli.CaptionsMasterDelay
	deadline := closeAt.Add(delay)
	for {
		decision := captions.Decide(captions.Situation{Policy: m.current, Origin: true, IngestCaptions: m.ingestSeen})
		tapReady := m.mediaFinished || m.parsedUntil >= req.EndMs
		recognitionReady := !decision.Recognize() || m.engine == nil || m.recognitionUnavailable || m.covered.UnixMilli() >= int64(req.EndMs)
		if m.current.Canonical == captions.CanonicalOff || (tapReady && recognitionReady) || !time.Now().Before(deadline) {
			break
		}
		changed := m.changed
		m.mu.Unlock()
		timer := time.NewTimer(time.Until(deadline))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
		m.mu.Lock()
	}
	p, ingest := m.current, m.ingestSeen
	until := m.signedUntil
	m.mu.Unlock()
	attachment := &muxl.TextAttachment{}
	if p.Canonical == captions.CanonicalOff {
		m.mu.Lock()
		m.signedUntil = req.EndMs
		m.pending = make(map[string]muxl.TextCue)
		m.mu.Unlock()
		return attachment, nil
	}
	if m.nextID == 0 {
		m.nextID = CaptionTrackIDBase
	}
	horizon := int64(req.StartMs) - captions.DefaultRetention.Milliseconds()
	from, to := time.UnixMilli(horizon), time.UnixMilli(int64(req.EndMs))
	for key, end := range m.consumed {
		if end <= horizon {
			delete(m.consumed, key)
		}
	}
	items := make(map[uint32]*muxl.TextTrackAttachment)
	for _, track := range m.hub.Tracks(m.streamer) {
		if track.Source == captions.SourceAuto && !strings.HasPrefix(track.ID, "push-") && (p.Canonical != captions.CanonicalAuto || ingest) {
			for key := range m.pending {
				if strings.HasPrefix(key, track.ID+"/") {
					delete(m.pending, key)
				}
			}
			continue
		}
		language := track.Language
		if (language == "" || language == "und") && track.Source == captions.SourceIngest && len(p.Languages) > 0 {
			language = p.Languages[0]
		}
		if language == "" {
			language = "und"
		}
		trackKey := string(track.Source) + "/" + strings.ToLower(language)
		for _, cue := range m.hub.Cues(m.streamer, track.ID, from, to) {
			key := track.ID + "/" + cue.ID
			if _, ok := m.consumed[key]; ok {
				continue
			}
			m.consumed[key] = cue.End.UnixMilli()
			start, end := cue.Start.UnixMilli(), cue.End.UnixMilli()
			if end <= 0 {
				continue
			}
			start = max(start, 0)
			// Live captions play in order on the canonical timeline. A late
			// cue starts in the first unsigned GoP, after the previous cue
			// has had its own reading time, and keeps its whole duration; it
			// replaces whatever the track still shows.
			duration := end - start
			start = max(start, int64(until), m.next[track.ID])
			m.next[track.ID] = start + duration
			end = start + duration
			if track.Source == captions.SourceAuto {
				end += autoCaptionHold.Milliseconds()
			}
			for k, shown := range m.pending {
				if strings.HasPrefix(k, track.ID+"/") && shown.End > uint64(start) {
					shown.End = uint64(start)
					m.pending[k] = shown
					if shown.End <= shown.Start {
						delete(m.pending, k)
					}
				}
			}
			m.pending[key] = muxl.TextCue{Start: uint64(start), End: uint64(end), Text: cue.Text, ID: m.sessionID + "/" + key}
			if _, ok := m.tracks[trackKey]; !ok {
				m.tracks[trackKey] = muxl.TextTrack{TrackID: m.nextID, Language: language, Label: string(track.Source)}
				m.nextID++
			}
		}
		config, ok := m.tracks[trackKey]
		if !ok {
			continue
		}
		item := items[config.TrackID]
		if item == nil {
			item = &muxl.TextTrackAttachment{TextTrack: config}
			items[config.TrackID] = item
		}
		for key, cue := range m.pending {
			if !strings.HasPrefix(key, track.ID+"/") {
				continue
			}
			if cue.Start < req.EndMs && cue.End > req.StartMs {
				clipped := cue
				clipped.Start = max(clipped.Start, req.StartMs)
				clipped.End = min(clipped.End, req.EndMs)
				item.Cues = append(item.Cues, clipped)
			}
			if cue.End <= req.EndMs {
				delete(m.pending, key)
			}
		}
	}
	for _, item := range items {
		attachment.Tracks = append(attachment.Tracks, *item)
	}
	m.mu.Lock()
	m.signedUntil = req.EndMs
	m.mu.Unlock()
	return attachment, nil
}

type captionManagerKey struct{}

func withCaptionManager(ctx context.Context, mm *MediaManager) context.Context {
	return context.WithValue(ctx, captionManagerKey{}, mm)
}
func (mm *MediaManager) registerCaptionMaster(streamer string, master captionSession) func() {
	mm.captionMasters.Store(streamer, master)
	return func() { mm.captionMasters.CompareAndDelete(streamer, master) }
}

// OriginCaptionPolicy reports whether this node owns a live ingest session.
func (mm *MediaManager) OriginCaptionPolicy(streamer string) (captions.Policy, bool) {
	v, ok := mm.captionMasters.Load(streamer)
	if !ok {
		return captions.Policy{}, false
	}
	p, err := v.(captionSession).captionPolicy()
	return p, err == nil
}

// PushCanonicalCaptions routes to that session's private master, never the node hub.
func (mm *MediaManager) PushCanonicalCaptions(streamer string, track captions.Track, cues []captions.Cue) error {
	v, ok := mm.captionMasters.Load(streamer)
	if !ok {
		return fmt.Errorf("stream not live")
	}
	return v.(captionSession).push(track, cues)
}

// SignOriginStream uses the node-wide engine and registers the session for
// pushCaptions while media is flowing through the streaming signer.
func (mm *MediaManager) SignOriginStream(ctx context.Context, ms MediaSigner, input io.Reader, events chan *muxl.MuxlEvent) error {
	return ms.SignSegmentStream(withCaptionManager(ctx, mm), input, events)
}
