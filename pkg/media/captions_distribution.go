package media

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/statedb"
)

type captionDistribution struct {
	mu       sync.Mutex
	ctx      context.Context
	writer   *records.Writer
	streams  map[string]*captionStream
	pending  map[string][]captions.Event
	closed   bool
	workers  sync.WaitGroup
	shutdown sync.Once
}

type captionSegment struct {
	vs        *validatedSegment
	segment   []byte
	media     time.Duration
	canonical bool
}

type captionStream struct {
	ctx            context.Context
	cancel         context.CancelFunc
	queue          chan captionSegment
	done           chan struct{}
	upstreamSignal chan struct{}
	// Protected by distribution.mu.
	upstream, canonical, published, recording bool
	policy                                    captions.Policy
	// Owned by the stream worker.
	decoder     *captionAudioDecoder
	recognizer  *captions.Recognizer
	recCancel   context.CancelFunc
	recDone     chan struct{}
	unavailable bool
}

func (mm *MediaManager) captionState() *captionDistribution {
	mm.captionDistributionOnce.Do(func() {
		mm.captionDistribution = &captionDistribution{
			ctx: context.Background(), streams: map[string]*captionStream{},
			pending: map[string][]captions.Event{},
		}
	})
	return mm.captionDistribution
}

// ConfigureCaptionRecords binds transcript writes to the stored streamer OAuth
// sessions and the node server repo, before any media arrives.
func (mm *MediaManager) ConfigureCaptionRecords(ctx context.Context, state *statedb.StatefulDB) error {
	d := mm.captionState()
	w, err := records.NewNodeWriter(mm.cli, mm.bus.Captions, mm.model, records.SessionClients(state))
	if err != nil {
		return err
	}
	d.ctx, d.writer = ctx, w
	go func() {
		<-ctx.Done()
		mm.ShutdownCaptions()
	}()
	return nil
}

func (mm *MediaManager) distributeCaptions(ctx context.Context, vs *validatedSegment, segment, header []byte) {
	if mm.bus == nil || mm.bus.Captions == nil {
		return
	}
	media, canonical, err := captions.SegmentClock(header, segment)
	if err != nil {
		log.Warn(ctx, "classify canonical captions", "error", err)
	}
	d := mm.captionState()
	d.mu.Lock()
	if d.closed || d.ctx.Err() != nil {
		d.mu.Unlock()
		return
	}
	s := d.streams[vs.repoDID]
	// Stop/go-live can leave the encoder connected in preview. A new live
	// session must not inherit the old cue IDs, clock, or transcript subject.
	if s != nil && s.published && !vs.meta.Published {
		mm.endCaptionSessionLocked(d, vs.repoDID)
		s = nil
	}
	if s == nil {
		sctx, cancel := context.WithCancel(context.WithoutCancel(d.ctx))
		s = &captionStream{
			ctx: sctx, cancel: cancel, queue: make(chan captionSegment, 32),
			upstreamSignal: make(chan struct{}, 1), done: make(chan struct{}),
			upstream: len(d.pending[vs.repoDID]) > 0,
		}
		d.streams[vs.repoDID] = s
		d.workers.Add(1)
		go func() { defer d.workers.Done(); mm.runCaptionStream(vs.repoDID, s) }()
	}
	if canonical {
		s.canonical = true
		mm.bus.Captions.RemoveOrigin(vs.repoDID, captions.OriginSidecar)
	}
	s.policy = captions.PolicyFromMetadata(vs.meta.MetadataConfiguration)
	s.published = vs.meta.Published
	if vs.meta.Published && !s.recording && d.writer != nil {
		d.writer.StartSessionWithOrigin(context.WithoutCancel(d.ctx), vs.repoDID, vs.meta.StartTime.Time(), vs.local)
		s.recording = true
	}
	select {
	case s.queue <- captionSegment{vs: vs, segment: segment, media: media, canonical: canonical && err == nil}:
	default:
		log.Warn(ctx, "caption distribution behind, dropping segment", "streamer", vs.repoDID)
	}
	d.mu.Unlock()
}

func (mm *MediaManager) runCaptionStream(streamer string, s *captionStream) {
	defer close(s.done)
	defer s.stopRecognition()
	for {
		var seg captionSegment
		select {
		case <-s.ctx.Done():
			d := mm.captionState()
			d.mu.Lock()
			drain := d.closed && d.streams[streamer] == s
			d.mu.Unlock()
			if !drain {
				return
			}
			select {
			case seg = <-s.queue:
			default:
				return
			}
		case <-s.upstreamSignal:
			s.stopRecognition()
			continue
		case seg = <-s.queue:
		}
		var events []captions.Event
		var err error
		if seg.canonical {
			events, err = captions.ReadCanonicalWithClock(context.WithoutCancel(s.ctx), seg.segment, seg.media, seg.vs.meta.StartTime.Time(), streamer)
		}
		if err != nil {
			log.Warn(s.ctx, "extract canonical captions", "error", err, "streamer", streamer)
		}
		d := mm.captionState()
		d.mu.Lock()
		if d.streams[streamer] != s || (s.ctx.Err() != nil && !d.closed) {
			d.mu.Unlock()
			return
		}
		incoming := s.canonical || s.upstream
		decision := captions.Decide(captions.Situation{Policy: s.policy, Origin: seg.vs.local, NodeCaptions: mm.cli.Captions, IncomingCaptions: incoming})
		if s.policy.AllowNodeCaptions && !s.canonical {
			for _, ev := range d.pending[streamer] {
				mm.bus.Captions.Publish(streamer, ev.Track, ev.Cue)
			}
		}
		delete(d.pending, streamer)
		for _, ev := range events {
			mm.bus.Captions.PublishCanonical(streamer, ev.Track, ev.Cue)
		}
		d.mu.Unlock()
		if s.ctx.Err() != nil || incoming || !decision.Recognize() || decision.Origin != captions.OriginSidecar {
			s.stopRecognition()
			continue
		}
		if s.unavailable || mm.STT == nil {
			continue
		}
		if s.recognizer == nil {
			if err := mm.startSidecar(streamer, s, decision); err != nil {
				s.unavailable = true
				log.Debug(s.ctx, "sidecar recognition unavailable", "error", err)
				continue
			}
		}
		if err := s.feedAudio(seg); err != nil {
			log.Warn(s.ctx, "sidecar audio decode", "error", err)
			s.stopRecognition()
			s.unavailable = true
		}
	}
}

func (mm *MediaManager) startSidecar(streamer string, s *captionStream, decision captions.Decision) error {
	// Gate publication separately from the recognizer: a pass already running
	// when an upstream track arrives must not publish a competing late final.
	hub := captions.NewHub(0)
	ctx, cancel := context.WithCancel(s.ctx)
	events := hub.Subscribe(ctx, streamer)
	r, err := captions.NewRecognizer(ctx, captions.RecognizerOptions{
		Streamer: streamer, Origin: captions.OriginSidecar, Author: mm.cli.ServerDID(),
		Languages: decision.Languages, Hub: hub, Engine: mm.STT,
	})
	if err != nil {
		cancel()
		return err
	}
	s.recognizer, s.recCancel, s.recDone = r, cancel, make(chan struct{})
	go func() {
		defer close(s.recDone)
		for ev := range events {
			d := mm.captionState()
			d.mu.Lock()
			if d.streams[streamer] == s && !s.upstream && !s.canonical && s.policy.AllowNodeCaptions && ctx.Err() == nil {
				mm.bus.Captions.Publish(streamer, ev.Track, ev.Cue)
			}
			d.mu.Unlock()
		}
	}()
	return nil
}

func (s *captionStream) stopRecognition() {
	if s.recCancel != nil {
		s.recCancel()
	}
	if s.decoder != nil {
		s.decoder.close()
		s.decoder = nil
	}
	if s.recognizer != nil {
		s.recognizer.Close()
		s.recognizer = nil
		<-s.recDone
		s.recCancel = nil
	}
}

func (s *captionStream) feedAudio(seg captionSegment) error {
	events, err := unwrapMuxlEvents(s.ctx, seg.segment)
	if err != nil {
		return err
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Audio == nil {
		return nil
	}
	var chosen, scale uint32
	var codec string
	for _, a := range cat.Audio.Renditions {
		if isAACCodec(a.Codec) {
			chosen, scale, codec = a.TrackID(), a.Timescale(), "aac"
			break
		}
		if isOpusCodec(a.Codec) {
			chosen, scale, codec = a.TrackID(), a.Timescale(), "opus"
		}
	}
	if chosen == 0 || scale == 0 {
		return nil
	}
	key := strconv.FormatUint(uint64(chosen), 10)
	audio := tracks[key]
	var tfdt uint64
	found := false
	for _, event := range events {
		if event.Type == "segment" || event.Type == "signed-segment" {
			if tfdt, found = event.FirstDecodeTimes[key]; found {
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("missing caption audio clock")
	}
	selected := map[string][]byte{strconv.FormatUint(uint64(chosen), 10): audio}
	if cat.Video != nil {
		for _, v := range cat.Video.Renditions {
			id := strconv.FormatUint(uint64(v.TrackID()), 10)
			selected[id] = tracks[id]
		}
	}
	decode := concatTracksByID(selected)
	if s.decoder != nil && (s.decoder.codec != codec || s.decoder.backwards(tfdt) || s.decoder.failed()) {
		s.decoder.close()
		s.decoder = nil
	}
	var init bytes.Buffer
	if s.decoder == nil {
		if err := muxl.RunMuxlWrapInit(s.ctx, bytes.NewReader(decode), &init); err != nil {
			return err
		}
		s.decoder, err = newCaptionAudioDecoder(s.ctx, codec, s.recognizer.Push)
		if err != nil {
			return err
		}
	}
	media := time.Duration(tfdt/uint64(scale))*time.Second + time.Duration(tfdt%uint64(scale))*time.Second/time.Duration(scale)
	return s.decoder.feed(init.Bytes(), decode, tfdt, media, seg.vs.meta.StartTime.Time())
}

// EndCaptionSession is the single teardown entry point, on timeout, shutdown,
// or the published livestream returning to preview. PDS I/O never blocks it.
func (mm *MediaManager) EndCaptionSession(streamer string) {
	d := mm.captionState()
	d.mu.Lock()
	mm.endCaptionSessionLocked(d, streamer)
	d.mu.Unlock()
}

func (mm *MediaManager) endCaptionSessionLocked(d *captionDistribution, streamer string) {
	s := d.streams[streamer]
	delete(d.streams, streamer)
	delete(d.pending, streamer)
	if s != nil {
		s.cancel()
	}
	if d.writer != nil {
		d.writer.FinishSession(streamer)
	}
	if mm.bus != nil && mm.bus.Captions != nil {
		mm.bus.Captions.EndSession(streamer)
	}
}

// CaptionSyndicationAllowed enforces the streamer's signed opt-out for both
// locally generated and passed-through sidecars.
func (mm *MediaManager) CaptionSyndicationAllowed(streamer string) bool {
	d := mm.captionState()
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.streams[streamer]
	return s != nil && s.policy.AllowNodeCaptions
}

// ReceiveSidecar accepts the connected upstream's stream and keeps its author.
// A short replay arriving before its first validated segment is held until the
// segment's signed policy has been checked; it is never exposed prematurely.
func (mm *MediaManager) ReceiveSidecar(streamer, upstream string, ev captions.Event) bool {
	if upstream == "" || upstream == mm.cli.ServerDID() || ev.Streamer != streamer || ev.Track.Origin != captions.OriginSidecar || ev.Cue.ID == "" || !ev.Cue.End.After(ev.Cue.Start) {
		return false
	}
	switch ev.Track.Source {
	case captions.SourceAuto, captions.SourceIngest, captions.SourceHuman, captions.SourceImported:
	default:
		return false
	}
	if ev.Track.Language == "" {
		ev.Track.Language = "und"
	}
	if len(ev.Track.Language) > 64 {
		return false
	}
	ev.Track.ID = captions.TrackID(captions.OriginSidecar, ev.Track.Source, ev.Track.Language)
	ev.Track.Author = upstream
	d := mm.captionState()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.ctx.Err() != nil {
		return false
	}
	s := d.streams[streamer]
	if s == nil {
		if len(d.pending[streamer]) < 256 {
			d.pending[streamer] = append(d.pending[streamer], ev)
		}
		return true
	}
	if !s.policy.AllowNodeCaptions || s.canonical {
		return false
	}
	s.upstream = true
	select {
	case s.upstreamSignal <- struct{}{}:
	default:
	}
	mm.bus.Captions.Publish(streamer, ev.Track, ev.Cue)
	return true
}

// ShutdownCaptions stops admission and waits for workers, including already
// ending sessions, before waiting for all transcript final flushes.
func (mm *MediaManager) ShutdownCaptions() {
	d := mm.captionState()
	d.shutdown.Do(func() {
		d.mu.Lock()
		d.closed = true
		for _, s := range d.streams {
			s.cancel()
		}
		d.mu.Unlock()
		d.workers.Wait()
		d.mu.Lock()
		for id := range d.streams {
			mm.endCaptionSessionLocked(d, id)
		}
		d.mu.Unlock()
		if d.writer != nil {
			d.writer.Stop()
		}
	})
}
