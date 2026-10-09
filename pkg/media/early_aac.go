package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

const earlySampleLimit = 512
const earlyByteLimit = 8 << 20
const earlySourceLimit = 16 << 20

type earlyAACSample struct {
	video  bool
	sample bus.PacketizedSample
}
type earlyAACPeer struct {
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan earlyAACSample
	bytes  int
}
type earlyAACSession struct {
	epoch        uint64
	user         string
	failureLog   sync.Once
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	feedMu       sync.Mutex
	cache        []earlyAACSample
	cacheBytes   int
	peers        map[*earlyAACPeer]struct{}
	err          error
	writer       *io.PipeWriter
	started      bool
	idle         *time.Timer
	idleDeadline time.Time
	closing      bool
	ended        bool
	encoderDone  chan struct{}
	epochState   *ingestEpochState
}

func (s *earlyAACSession) logFailure(err error) {
	s.failureLog.Do(func() {
		log.Log(context.Background(), "experimental early AAC session failed", "user", s.user, "epoch", s.epoch, "reason", err)
	})
}

func (s *earlyAACSession) fail(err error) {
	if err == io.EOF {
		s.finish()
		return
	}
	s.mu.Lock()
	if s.err == nil {
		s.err = err
		s.logFailure(err)
		s.cache = nil
		s.cacheBytes = 0
		for p := range s.peers {
			p.cancel()
			delete(s.peers, p)
		}
		s.cancel()
		s.writer.CloseWithError(err)
	}
	s.mu.Unlock()
}
func (s *earlyAACSession) finish() {
	s.mu.Lock()
	if !s.ended && s.err == nil {
		s.closing = true
		s.ended = true
		s.cache = nil
		s.cacheBytes = 0
		for p := range s.peers {
			close(p.queue)
		}
	}
	s.mu.Unlock()
}

func (s *earlyAACSession) closeOrderly() {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	s.writer.Close()
	if s.encoderDone != nil {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-s.encoderDone:
		case <-timer.C:
			s.fail(fmt.Errorf("early AAC encoder drain timeout"))
			return
		}
	} else {
		s.finish()
	}
}

func (s *earlyAACSession) publish(item earlyAACSample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.ended {
		return
	}
	size := len(item.sample.Data)
	if len(s.cache) >= earlySampleLimit || s.cacheBytes+size > earlyByteLimit {
		s.err = fmt.Errorf("early AAC cache overflow")
		s.logFailure(s.err)
		s.cache = nil
		s.cacheBytes = 0
		s.cancel()
		s.writer.CloseWithError(s.err)
		for p := range s.peers {
			p.cancel()
			delete(s.peers, p)
		}
		return
	}
	s.cache = append(s.cache, item)
	s.cacheBytes += size
	for p := range s.peers {
		if p.ctx.Err() != nil {
			delete(s.peers, p)
			continue
		}
		if p.bytes+size > earlyByteLimit {
			p.cancel()
			delete(s.peers, p)
			continue
		}
		select {
		case p.queue <- item:
			p.bytes += size
		default:
			p.cancel()
			delete(s.peers, p)
		}
	}
}
func (s *earlyAACSession) join() *earlyAACPeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.ctx.Err() != nil || s.closing {
		return nil
	}
	var video, audio bool
	var start time.Duration
	for _, item := range s.cache {
		if item.video && !video {
			if !auHasIDR(item.sample.Data) {
				return nil
			}
			video = true
			start = item.sample.Timestamp
		}
	}
	if video {
		for _, item := range s.cache {
			if !item.video && item.sample.Timestamp >= start {
				audio = true
				break
			}
		}
	}
	if !video || !audio {
		return nil
	}
	ctx, cancel := context.WithCancel(s.ctx)
	p := &earlyAACPeer{ctx: ctx, cancel: cancel, queue: make(chan earlyAACSample, earlySampleLimit)}
	for _, wantVideo := range []bool{true, false} {
		for _, item := range s.cache {
			if item.video == wantVideo && item.sample.Timestamp >= start {
				p.queue <- item
				p.bytes += len(item.sample.Data)
			}
		}
	}
	s.peers[p] = struct{}{}
	return p
}
func (s *earlyAACSession) take(p *earlyAACPeer, item earlyAACSample) {
	s.mu.Lock()
	p.bytes -= len(item.sample.Data)
	s.mu.Unlock()
}
func (s *earlyAACSession) leave(p *earlyAACPeer) {
	s.mu.Lock()
	delete(s.peers, p)
	p.cancel()
	for {
		select {
		case _, ok := <-p.queue:
			if !ok {
				p.bytes = 0
				s.mu.Unlock()
				return
			}
		default:
			p.bytes = 0
			s.mu.Unlock()
			return
		}
	}
}

func newEarlyAACSession(ctx context.Context, epoch uint64, user string) (*earlyAACSession, error) {
	ctx, cancel := context.WithCancel(ctx)
	p, err := gst.NewPipelineFromString("appsrc name=src ! qtdemux name=demux\ndemux.video_0 ! " + constants.Queue2Big + " ! fakesink sync=false async=false\ndemux.audio_0 ! " + constants.Queue2Big + " ! aacparse ! fdkaacdec ! audioconvert ! audioresample ! opusenc ! appsink name=opus sync=false")
	if err != nil {
		cancel()
		return nil, err
	}
	r, w := io.Pipe()
	s := &earlyAACSession{epoch: epoch, ctx: ctx, cancel: cancel, writer: w, peers: map[*earlyAACPeer]struct{}{}, encoderDone: make(chan struct{})}
	s.user = user
	src, err := p.GetElementByName("src")
	if err != nil {
		cancel()
		return nil, err
	}
	app.SrcFromElement(src).SetCallbacks(&app.SourceCallbacks{NeedDataFunc: ReaderNeedDataIncremental(ctx, r)})
	sink, err := p.GetElementByName("opus")
	if err != nil {
		cancel()
		return nil, err
	}
	var end time.Duration
	first := true
	app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowEOS
		}
		b := sample.GetBuffer()
		ts, dur := b.PresentationTimestamp().AsDuration(), b.Duration().AsDuration()
		if ts == nil || dur == nil || *dur <= 0 || (!first && (*ts-end > time.Millisecond || end-*ts > time.Millisecond)) || (first && (*ts != 0 || *dur != 13500*time.Microsecond)) {
			s.fail(fmt.Errorf("early AAC invalid Opus timing: pts=%v duration=%v prior-end=%s first=%v", ts, dur, end, first))
			return gst.FlowError
		}
		mapped := b.Map(gst.MapRead)
		if mapped == nil {
			s.fail(fmt.Errorf("early AAC unmappable Opus"))
			return gst.FlowError
		}
		data := append([]byte(nil), mapped.Bytes()...)
		b.Unmap()
		if len(data) == 0 || len(data) > 65536 {
			s.fail(fmt.Errorf("early AAC invalid Opus payload"))
			return gst.FlowError
		}
		first = false
		end = *ts + *dur
		s.publish(earlyAACSample{sample: bus.PacketizedSample{Data: data, Duration: *dur, Timestamp: *ts, HasTimestamp: true}})
		return gst.FlowOK
	}})
	go func() {
		err := HandleBusMessages(ctx, p)
		if err == nil {
			err = io.EOF
		}
		s.fail(err)
		r.CloseWithError(err)
		if err := p.SetState(gst.StateNull); err != nil {
			log.Error(ctx, "early AAC converter cleanup", "error", err)
		}
		close(s.encoderDone)
	}()
	go func() {
		select {
		case <-ctx.Done():
			w.CloseWithError(ctx.Err())
			r.CloseWithError(ctx.Err())
		case <-s.encoderDone:
		}
	}()
	if err = p.SetState(gst.StatePlaying); err != nil {
		s.fail(err)
		return nil, err
	}
	return s, nil
}
func (s *earlyAACSession) feed(ctx context.Context, source []byte, video []bus.PacketizedSample) error {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	if len(source) > earlySourceLimit {
		s.fail(fmt.Errorf("early AAC source overflow"))
		return s.ctx.Err()
	}
	if s.ctx.Err() != nil {
		s.mu.Lock()
		err := s.err
		s.mu.Unlock()
		return err
	}
	if len(video) == 0 || !auHasIDR(video[0].Data) {
		s.fail(fmt.Errorf("early AAC startup video is not IDR"))
		return s.ctx.Err()
	}
	// Keep just the verified IDR GOP and its
	// available audio for startup; existing peers retain their queued packets.
	s.mu.Lock()
	if len(video) > 0 {
		start := video[0].Timestamp
		kept := make([]earlyAACSample, 0, len(s.cache))
		s.cacheBytes = 0
		for _, item := range s.cache {
			if !item.video && item.sample.Timestamp >= start {
				kept = append(kept, item)
				s.cacheBytes += len(item.sample.Data)
			}
		}
		s.cache = kept
	}
	s.mu.Unlock()
	for _, v := range video {
		if !v.HasTimestamp || v.Duration <= 0 {
			s.fail(fmt.Errorf("early AAC missing video timing"))
			return s.ctx.Err()
		}
		s.publish(earlyAACSample{video: true, sample: v})
	}
	done := make(chan error, 1)
	go func() {
		if !s.started {
			var init bytes.Buffer
			if err := muxl.RunMuxlWrapInit(s.ctx, bytes.NewReader(source), &init); err != nil {
				done <- err
				return
			}
			if _, err := s.writer.Write(init.Bytes()); err != nil {
				done <- err
				return
			}
			s.started = true
		}
		_, err := s.writer.Write(source)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			s.fail(err)
		}
		return err
	case <-ctx.Done():
		s.fail(ctx.Err())
		<-done
		return ctx.Err()
	case <-s.ctx.Done():
		<-done
		return s.ctx.Err()
	}
}

func earlyMediaDuration(ticks uint64, scale uint32) (time.Duration, error) {
	if scale == 0 {
		return 0, fmt.Errorf("early AAC zero timescale")
	}
	seconds, remainder := ticks/uint64(scale), ticks%uint64(scale)
	if seconds > uint64(math.MaxInt64)/uint64(time.Second) {
		return 0, fmt.Errorf("early AAC unrepresentable media duration")
	}
	nanos := seconds*uint64(time.Second) + remainder*uint64(time.Second)/uint64(scale)
	if nanos > math.MaxInt64 {
		return 0, fmt.Errorf("early AAC unrepresentable media duration")
	}
	return time.Duration(nanos), nil
}

func restoreEarlyVideoOrigin(events []*muxl.MuxlEvent, cat *muxl.MuxlCatalog, video []bus.PacketizedSample) error {
	if cat == nil || cat.Video == nil || len(video) == 0 {
		return fmt.Errorf("early AAC missing video")
	}
	for _, v := range cat.Video.Renditions {
		tid := fmt.Sprint(v.TrackID())
		scale := v.Timescale()
		if scale == 0 {
			return fmt.Errorf("early AAC zero video timescale")
		}
		for _, event := range events {
			if start, ok := event.FirstDecodeTimes[tid]; ok {
				origin, err := earlyMediaDuration(uint64(start), scale)
				if err != nil {
					return err
				}
				relative := video[0].Timestamp
				for i := range video {
					if !video[i].HasTimestamp {
						return fmt.Errorf("early AAC missing video timestamp")
					}
					if relative < 0 || video[i].Timestamp < relative {
						return fmt.Errorf("early AAC invalid relative video time")
					}
					delta := video[i].Timestamp - relative
					if delta > time.Duration(math.MaxInt64)-origin {
						return fmt.Errorf("early AAC unrepresentable video timestamp")
					}
					video[i].Timestamp = origin + delta
				}
				return nil
			}
		}
	}
	return fmt.Errorf("early AAC missing verified video origin")
}

func (mm *MediaManager) removeEarlyEpoch(ctx context.Context) *earlyAACSession {
	state := ingestState(ctx)
	if state == nil {
		return nil
	}
	state.mu.Lock()
	state.retired = true
	did := state.did
	state.mu.Unlock()
	mm.earlyMu.Lock()
	defer mm.earlyMu.Unlock()
	s := mm.earlySessions[did]
	if s == nil || s.epoch != state.epoch {
		return nil
	}
	delete(mm.earlySessions, did)
	if s.idle != nil {
		s.idle.Stop()
	}
	return s
}
func (mm *MediaManager) closeEarlyEpoch(ctx context.Context) {
	if s := mm.removeEarlyEpoch(ctx); s != nil {
		s.closeOrderly()
	}
}
func (mm *MediaManager) abortEarlyEpoch(ctx context.Context) {
	if s := mm.removeEarlyEpoch(ctx); s != nil {
		s.fail(fmt.Errorf("early AAC source validation failed"))
	}
}
func (mm *MediaManager) prepareEarlyAAC(ctx context.Context, did string) bool {
	state := ingestState(ctx)
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.retired || (state.did != "" && state.did != did) {
		return false
	}
	mm.earlyMu.Lock()
	defer mm.earlyMu.Unlock()
	s := mm.earlySessions[did]
	if s != nil {
		s.mu.Lock()
		unavailable := state.epoch < s.epoch || (state.epoch == s.epoch && (s.err != nil || s.closing))
		s.mu.Unlock()
		if unavailable {
			return false
		}
	}
	state.did = did
	return true
}

func (mm *MediaManager) feedEarlyAAC(ctx context.Context, vs *validatedSegment, source []byte) {
	epoch := ingestSessionFromContext(ctx)
	if !mm.cli.ExperimentalEarlyAACPlayback || mm.cli.MaximumLiveBitrate != 0 || mm.bus == nil || epoch == 0 {
		return
	}
	if !vs.meta.Published {
		mm.abortEarlyEpoch(ctx)
		return
	}
	if !mm.prepareEarlyAAC(ctx, vs.repoDID) {
		return
	}
	if len(source) > earlySourceLimit {
		mm.abortEarlyEpoch(ctx)
		return
	}
	// Packetize only original verified video: AAC must not enter the Opus writer.
	events, err := unwrapMuxlEvents(ctx, source)
	if err != nil {
		mm.abortEarlyEpoch(ctx)
		return
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil {
		mm.abortEarlyEpoch(ctx)
		return
	}
	var videoData []byte
	for _, v := range cat.Video.Renditions {
		videoData = tracks[fmt.Sprint(v.TrackID())]
		break
	}
	var flat bytes.Buffer
	if err = muxl.RunMuxlWrap(ctx, bytes.NewReader(videoData), "flat", &flat); err != nil {
		mm.abortEarlyEpoch(ctx)
		return
	}
	packet, err := Packetize(ctx, mm.cli, &bus.Seg{Data: flat.Bytes()})
	if err != nil || len(packet.Video) == 0 {
		mm.abortEarlyEpoch(ctx)
		return
	}
	if err = restoreEarlyVideoOrigin(events, cat, packet.Video); err != nil {
		mm.abortEarlyEpoch(ctx)
		return
	}
	state := ingestState(ctx)
	state.mu.Lock()
	if state.retired {
		state.mu.Unlock()
		return
	}
	mm.earlyMu.Lock()
	state.mu.Unlock()
	if mm.earlySessions == nil {
		mm.earlySessions = map[string]*earlyAACSession{}
	}
	s := mm.earlySessions[vs.repoDID]
	if s != nil && epoch < s.epoch {
		mm.earlyMu.Unlock()
		return
	}
	if s == nil || epoch > s.epoch {
		if s != nil {
			s.fail(fmt.Errorf("early AAC ingest replaced"))
			if s.idle != nil {
				s.idle.Stop()
			}
		}
		s, err = newEarlyAACSession(context.WithoutCancel(ctx), epoch, vs.repoDID)
		if err != nil {
			mm.earlyMu.Unlock()
			return
		}
		s.epochState = ingestState(ctx)
		go func(current *earlyAACSession) {
			<-current.encoderDone
			state := current.epochState
			state.mu.Lock()
			state.retired = true
			state.mu.Unlock()
			mm.earlyMu.Lock()
			if mm.earlySessions[vs.repoDID] == current {
				delete(mm.earlySessions, vs.repoDID)
				if current.idle != nil {
					current.idle.Stop()
				}
			}
			mm.earlyMu.Unlock()
		}(s)
		mm.earlySessions[vs.repoDID] = s
	}
	if s.idle != nil {
		s.idle.Stop()
	}
	s.idleDeadline = time.Now().Add(streamTranscoderIdle)
	s.idle = time.AfterFunc(streamTranscoderIdle, func() { mm.reapEarlyAAC(vs.repoDID, s) })
	mm.earlyMu.Unlock()
	feedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = s.feed(feedCtx, source, packet.Video)
}
func (mm *MediaManager) reapEarlyAAC(user string, s *earlyAACSession) {
	mm.earlyMu.Lock()
	defer mm.earlyMu.Unlock()
	if mm.earlySessions[user] == s && !time.Now().Before(s.idleDeadline) {
		s.fail(fmt.Errorf("early AAC ingest idle"))
	}
}

func (mm *MediaManager) joinEarlyAAC(user, rendition string) (*earlyAACSession, *earlyAACPeer) {
	if !mm.cli.ExperimentalEarlyAACPlayback || mm.cli.MaximumLiveBitrate != 0 || rendition != "source" {
		return nil, nil
	}
	mm.earlyMu.Lock()
	defer mm.earlyMu.Unlock()
	s := mm.earlySessions[user]
	if s == nil {
		return nil, nil
	}
	p := s.join()
	if p == nil {
		return nil, nil
	}
	return s, p
}
