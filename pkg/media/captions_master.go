package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
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

// liveTapWait bounds how long a live GoP waits for the ingest tap to parse it,
// so embedded captions land in their own GoP. Live media never waits for
// speech recognition; the archive pass places late words for recordings.
const liveTapWait = 500 * time.Millisecond

// archiveTimingSlack absorbs whisper re-timing words between passes: a word a
// later pass commits can start slightly before audio an earlier pass already
// finalized, so the archive pass waits for coverage a little past its GoP.
const archiveTimingSlack = time.Second

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
	pushed                 bool
	recognitionUnavailable bool
	parsedUntil            uint64
	mediaFinished          bool
	arrival                time.Time
	mediaOrigin            time.Time
	covered                time.Time
	changed                chan struct{}
	closes                 map[uint64]time.Time
	gopTimes               map[uint64]time.Time
	tracks                 map[string]muxl.TextTrack
	nextID                 uint32
	live                   captionLayout
	archive                *captionArchivePass
}

// captionLayout places final cues on one output's timeline. The live stream
// and the archive pass keep separate layouts over the same cues.
type captionLayout struct {
	until    uint64 // end of the last GoP laid out; written under captionMaster.mu
	consumed map[string]int64
	pending  map[string]muxl.TextCue
	next     map[string]int64 // per track, the earliest start of its next cue
	// keepTimes starts every cue that arrives in time at its own time, even
	// if that cuts short a cue still showing, as long as it doesn't erase
	// it. Without it a cue also waits for the reading time of the cue before
	// it, which keeps live captions readable but lets one late cue delay
	// everything after it.
	keepTimes bool
}

func newCaptionLayout(keepTimes bool) captionLayout {
	return captionLayout{consumed: make(map[string]int64), pending: make(map[string]muxl.TextCue), next: make(map[string]int64), keepTimes: keepTimes}
}

// erases reports whether a cue starting at start would wipe out one of the
// track's pending cues, as when whisper times a cue to start with the one
// before it.
func (l *captionLayout) erases(trackID string, start int64) bool {
	for key, shown := range l.pending {
		if strings.HasPrefix(key, trackID+"/") && shown.Start >= uint64(start) {
			return true
		}
	}
	return false
}

func newCaptionMaster(ctx context.Context, streamer string, cli *config.CLI, engine stt.Engine) *captionMaster {
	return &captionMaster{ctx: ctx, streamer: streamer, sessionID: uuid.NewString(), cli: cli, engine: engine, hub: captions.NewHub(0), current: captions.DefaultPolicy(), changed: make(chan struct{}), closes: make(map[uint64]time.Time), gopTimes: make(map[uint64]time.Time), tracks: make(map[string]muxl.TextTrack), nextID: CaptionTrackIDBase, live: newCaptionLayout(false)}
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

// waitLocked waits, with m.mu held on entry and on return, until the master
// changes, deadline passes (a zero deadline never does), or ctx ends.
func (m *captionMaster) waitLocked(ctx context.Context, deadline time.Time) error {
	changed := m.changed
	m.mu.Unlock()
	defer m.mu.Lock()
	var timeout <-chan time.Time
	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-changed:
	case <-timeout:
	}
	return nil
}

// stop ends the session: no more media will be signed, and the archive pass
// drains what was.
func (m *captionMaster) stop() {
	m.mu.Lock()
	m.stopped = true
	m.signal()
	m.mu.Unlock()
}
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
	// muxl asks for the time right after the GoP's text, which completes the
	// GoP the archive pass will lay out again.
	if a := m.archive; a != nil && a.signed != nil && a.signed.req.StartMs == start {
		gop := *a.signed
		gop.when = prediction
		a.gops = append(a.gops, gop)
		a.signed = nil
		m.signal()
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
	if end > m.live.until {
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
	m.pushed = true
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
// from a private hub. Live media waits only for the ingest tap: a final that
// arrives after its GoP was signed is carried into the next unsigned GoP, and
// the archive pass puts it back in its own GoP for recordings.
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
	deadline := closeAt.Add(liveTapWait)
	for m.current.Canonical != captions.CanonicalOff && !m.mediaFinished && m.parsedUntil < req.EndMs && time.Now().Before(deadline) {
		if err := m.waitLocked(ctx, deadline); err != nil {
			m.mu.Unlock()
			return nil, err
		}
	}
	m.mu.Unlock()
	attachment := m.layout(&m.live, req)
	if m.archive != nil {
		m.mu.Lock()
		m.archive.signed = &archiveGoP{req: req, closeAt: closeAt, live: attachment}
		m.mu.Unlock()
	}
	return attachment, nil
}

// layout places the final cues overlapping req on l's timeline. Cues play in
// order: one that arrives after its GoP was laid out starts in the next GoP,
// after the previous cue has had its own reading time, and keeps its whole
// duration; it replaces whatever the track still shows. Every declared track
// appears in every GoP, so neither output drops a track mid-stream.
func (m *captionMaster) layout(l *captionLayout, req muxl.TextRequest) *muxl.TextAttachment {
	m.mu.Lock()
	p, ingest := m.current, m.ingestSeen
	m.mu.Unlock()
	cues := make(map[uint32][]muxl.TextCue)
	if p.Canonical == captions.CanonicalOff {
		l.pending = make(map[string]muxl.TextCue)
	} else {
		horizon := int64(req.StartMs) - captions.DefaultRetention.Milliseconds()
		from, to := time.UnixMilli(horizon), time.UnixMilli(int64(req.EndMs))
		for key, end := range l.consumed {
			if end <= horizon {
				delete(l.consumed, key)
			}
		}
		for _, track := range m.hub.Tracks(m.streamer) {
			if track.Source == captions.SourceAuto && !strings.HasPrefix(track.ID, "push-") && (p.Canonical != captions.CanonicalAuto || ingest) {
				for key := range l.pending {
					if strings.HasPrefix(key, track.ID+"/") {
						delete(l.pending, key)
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
				if _, ok := l.consumed[key]; ok {
					continue
				}
				l.consumed[key] = cue.End.UnixMilli()
				start, end := cue.Start.UnixMilli(), cue.End.UnixMilli()
				if end <= 0 {
					continue
				}
				start = max(start, 0)
				duration := end - start
				if !l.keepTimes || start < int64(l.until) || l.erases(track.ID, start) {
					start = max(start, int64(l.until), l.next[track.ID])
				}
				l.next[track.ID] = start + duration
				end = start + duration
				if track.Source == captions.SourceAuto {
					end += autoCaptionHold.Milliseconds()
				}
				for k, shown := range l.pending {
					if strings.HasPrefix(k, track.ID+"/") && shown.End > uint64(start) {
						shown.End = uint64(start)
						l.pending[k] = shown
						if shown.End <= shown.Start {
							delete(l.pending, k)
						}
					}
				}
				l.pending[key] = muxl.TextCue{Start: uint64(start), End: uint64(end), Text: cue.Text, ID: m.sessionID + "/" + key}
				m.declareTrack(trackKey, language, track.Source)
			}
			m.mu.Lock()
			config, ok := m.tracks[trackKey]
			m.mu.Unlock()
			if !ok {
				continue
			}
			for key, cue := range l.pending {
				if !strings.HasPrefix(key, track.ID+"/") {
					continue
				}
				if cue.Start < req.EndMs && cue.End > req.StartMs {
					clipped := cue
					clipped.Start = max(clipped.Start, req.StartMs)
					clipped.End = min(clipped.End, req.EndMs)
					cues[config.TrackID] = append(cues[config.TrackID], clipped)
				}
				if cue.End <= req.EndMs {
					delete(l.pending, key)
				}
			}
		}
	}
	attachment := &muxl.TextAttachment{}
	m.mu.Lock()
	l.until = req.EndMs
	for _, config := range m.tracks {
		attachment.Tracks = append(attachment.Tracks, muxl.TextTrackAttachment{TextTrack: config, Cues: cues[config.TrackID]})
	}
	m.mu.Unlock()
	sort.Slice(attachment.Tracks, func(i, j int) bool { return attachment.Tracks[i].TrackID < attachment.Tracks[j].TrackID })
	for _, track := range attachment.Tracks {
		sort.Slice(track.Cues, func(i, j int) bool {
			a, b := track.Cues[i], track.Cues[j]
			if a.Start != b.Start {
				return a.Start < b.Start
			}
			if a.End != b.End {
				return a.End < b.End
			}
			return a.ID < b.ID
		})
	}
	return attachment
}

// declareTrack gives a source and language their track ID on first use. IDs
// and configurations never change during the session, in either output.
func (m *captionMaster) declareTrack(key, language string, source captions.Source) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tracks[key]; !ok {
		m.tracks[key] = muxl.TextTrack{TrackID: m.nextID, Language: language, Label: string(source)}
		m.nextID++
	}
}

// captionArchivePass lays each live-signed GoP out again once recognition has
// covered it, or CaptionsMasterDelay after it closed, so a recording shows
// words in the GoP they were spoken in rather than where they arrived live.
type captionArchivePass struct {
	layout captionLayout
	signed *archiveGoP  // laid out live, until segmentTime stamps it
	gops   []archiveGoP // stamped and awaiting the pass; guarded by captionMaster.mu
	signer muxl.SignerInput
	put    func(archiveText) error
	done   chan struct{}
}

type archiveGoP struct {
	req     muxl.TextRequest
	closeAt time.Time
	when    time.Time // the GoP's signed start time
	live    *muxl.TextAttachment
}

// archiveTo starts the archive pass. signer carries the cert, the key or Sign
// callback, and the manifest for its text runs; put receives every GoP the
// streaming signer signed, in order.
func (m *captionMaster) archiveTo(signer muxl.SignerInput, put func(archiveText) error) {
	m.archive = &captionArchivePass{layout: newCaptionLayout(true), signer: signer, put: put, done: make(chan struct{})}
	go m.runArchive()
}

// awaitArchive waits until the archive pass has handed over every signed GoP.
// It drains once the session stops.
func (m *captionMaster) awaitArchive() {
	if m.archive != nil {
		<-m.archive.done
	}
}

func (m *captionMaster) runArchive() {
	a := m.archive
	defer close(a.done)
	for {
		m.mu.Lock()
		for len(a.gops) == 0 && !m.stopped {
			_ = m.waitLocked(context.Background(), time.Time{})
		}
		if len(a.gops) == 0 {
			m.mu.Unlock()
			return
		}
		gop := a.gops[0]
		a.gops = a.gops[1:]
		deadline := gop.closeAt.Add(m.cli.CaptionsMasterDelay)
		for !m.archiveReady(gop.req.EndMs) && time.Now().Before(deadline) {
			if m.waitLocked(m.ctx, deadline) != nil {
				break
			}
		}
		m.mu.Unlock()
		text := archiveText{StartMs: gop.req.StartMs}
		if attachment := m.layout(&a.layout, gop.req); !reflect.DeepEqual(attachment, gop.live) {
			in := a.signer
			in.SegmentTimeFn = func(uint64) time.Time { return gop.when }
			runs, err := muxl.RunMuxlSignTextRuns(context.WithoutCancel(m.ctx), gop.req, attachment.Tracks, in)
			if err != nil {
				log.Warn(m.ctx, "sign archive captions; recording the live text", "error", err, "streamer", m.streamer)
			} else {
				text.Runs = runs
			}
		}
		if err := a.put(text); err != nil {
			log.Warn(m.ctx, "hand over archive captions", "error", err, "streamer", m.streamer)
		}
	}
}

// archiveReady reports, with m.mu held, whether the archive pass has every
// caption for the GoP ending at end: recognition has covered it, nothing
// recognizes it, or the media ended. Pushed captions give no such signal, so
// a session that has had pushes waits out the hold.
func (m *captionMaster) archiveReady(end uint64) bool {
	if m.mediaFinished || m.current.Canonical == captions.CanonicalOff {
		return true
	}
	if m.pushed || m.parsedUntil < end {
		return false
	}
	decision := captions.Decide(captions.Situation{Policy: m.current, Origin: true, IngestCaptions: m.ingestSeen})
	return !decision.Recognize() || m.engine == nil || m.recognitionUnavailable || m.covered.UnixMilli() >= int64(end)+archiveTimingSlack.Milliseconds()
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
