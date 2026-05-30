package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// transcodeJob is one fed source segment awaiting its transcoded twin. token is
// opaque caller context (the validated-segment info), returned verbatim to
// onComplete so the caller can archive/distribute the completed segment.
type transcodeJob struct {
	src   []byte
	token any
}

// streamTranscoder runs ONE long-lived audio transcode for a single stream.
//
// Source segments are fed (via [streamTranscoder.Feed]) in arrival order into a
// continuous decode→encode pipeline whose output is re-segmented at the
// source's own video keyframes (muxl's segmenter, with the passed-through video
// as the cut clock). Because the encoder runs continuously, the transcoded
// audio is gapless — encoder priming happens once at stream start, not per
// segment — and the re-segmentation is a lossless regroup at boundaries
// identical to the source's. Each completed dual-codec segment is delivered to
// onComplete in order, ~1 GoP behind the fed segment (the encoder needs the
// next segment's head to finish the current one).
//
// Lifecycle: created per stream, Feed in order, Close on stream end (flushes the
// tail). Not safe for concurrent Feed; feed from one goroutine (the stream's
// validate path delivers segments in order).
type streamTranscoder struct {
	mm         *MediaManager
	target     string // codec being ADDED: "opus" (source AAC) or "aac" (source Opus)
	cert       []byte
	keyPEM     []byte
	onComplete func(token any, completed []byte)

	ctx    context.Context
	cancel context.CancelFunc
	feedW  *io.PipeWriter
	jobs   chan transcodeJob
	done   chan struct{}

	feedMu sync.Mutex  // serializes Feed so segments enter in order
	reaper *time.Timer // idle reaper; reset on Feed (set by the registry)
	fedSeq int         // per-instance feed counter (debug dump ordering); under feedMu

	mu      sync.Mutex
	started bool
	closed  bool
	err     error
}

// streamTranscoderIdle is how long a per-stream transcoder lingers with no new
// segment before it's flushed and torn down (the stream is presumed ended).
const streamTranscoderIdle = 30 * time.Second

// feedStreamTranscoder routes one source segment into the stream's continuous
// transcoder, creating it on first use. The completed dual-codec segment is
// distributed asynchronously (≈1 GoP later) via distributeSegment.
func (mm *MediaManager) feedStreamTranscoder(ctx context.Context, vs *validatedSegment, src []byte, target string, cert, keyPEM []byte) error {
	did := vs.repoDID
	mm.transcodersMu.Lock()
	t := mm.transcoders[did]
	if t != nil && t.needsReset(target) {
		// Source codec swapped (the needed target flipped, e.g. a streamer
		// dropped RTMP/AAC and picked up WHIP/Opus) or the pipeline failed. The
		// old pipeline expects the previous source codec, so feeding it the new
		// one would stall it. Flush + tear it down (async, so we don't block
		// ingest — its tail segments still complete) and rebuild for the new
		// codec. One seam at the swap, clean after.
		old := t
		delete(mm.transcoders, did)
		t = nil
		log.Log(ctx, "stream source codec swapped, resetting transcoder",
			"streamer", did, "from_target", old.target, "to_target", target)
		go func() {
			if err := old.Close(); err != nil {
				log.Error(ctx, "stream transcoder reset close failed", "streamer", did, "error", err)
			}
		}()
	}
	if t == nil {
		// The transcoder outlives any single request; carry log values but not
		// cancellation, and cancel explicitly on reap.
		streamCtx := context.WithoutCancel(ctx)
		t = mm.newStreamTranscoder(streamCtx, target, cert, keyPEM, func(token any, completed []byte) {
			v := token.(*validatedSegment)
			if err := mm.distributeSegment(context.WithoutCancel(streamCtx), v, completed); err != nil {
				log.Error(streamCtx, "distribute completed segment failed", "streamer", v.repoDID, "error", err)
			}
		})
		t.reaper = time.AfterFunc(streamTranscoderIdle, func() { mm.reapStreamTranscoder(did, t) })
		mm.transcoders[did] = t
		log.Log(ctx, "stream transcoder started", "streamer", did, "target", target)
	}
	mm.transcodersMu.Unlock()

	t.reaper.Reset(streamTranscoderIdle)
	return t.Feed(src, vs)
}

// needsReset reports whether an existing per-stream transcoder must be torn
// down and rebuilt: the source codec swapped (target flipped — e.g. RTMP/AAC →
// WHIP/Opus mid-stream) or its pipeline has failed.
func (t *streamTranscoder) needsReset(target string) bool {
	return t.target != target || t.failed()
}

// failed reports whether the transcoder's pipeline has errored out.
func (t *streamTranscoder) failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err != nil
}

// reapStreamTranscoder flushes and removes an idle stream's transcoder (the
// flush emits its final buffered segment). Safe to call once per timer fire.
func (mm *MediaManager) reapStreamTranscoder(did string, t *streamTranscoder) {
	mm.transcodersMu.Lock()
	if mm.transcoders[did] == t {
		delete(mm.transcoders, did)
	}
	mm.transcodersMu.Unlock()
	if err := t.Close(); err != nil {
		log.Error(t.ctx, "stream transcoder reap close failed", "streamer", did, "error", err)
	}
}

// newStreamTranscoder starts a per-stream continuous transcoder. target is the
// codec to add. cert/keyPEM are the node's S2PA signer. onComplete is invoked,
// in order, with each completed dual-codec segment (and its feed token).
func (mm *MediaManager) newStreamTranscoder(parent context.Context, target string, cert, keyPEM []byte, onComplete func(token any, completed []byte)) *streamTranscoder {
	ctx, cancel := context.WithCancel(parent)
	feedR, feedW := io.Pipe()
	t := &streamTranscoder{
		mm: mm, target: target, cert: cert, keyPEM: keyPEM, onComplete: onComplete,
		ctx: ctx, cancel: cancel, feedW: feedW,
		jobs: make(chan transcodeJob, 16), done: make(chan struct{}),
	}
	go func() {
		defer close(t.done)
		err := t.run(feedR)
		t.mu.Lock()
		if t.err == nil {
			t.err = err
		}
		t.mu.Unlock()
		feedR.CloseWithError(err) // unblock the appsrc feeder if still writing
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error(ctx, "stream transcoder exited", "target", target, "error", err)
		}
	}()
	return t
}

// Feed pushes a source segment (bare canonical .m4s) in arrival order. The
// matching completed segment is delivered to onComplete later (≈1 GoP behind).
func (t *streamTranscoder) Feed(src []byte, token any) error {
	t.feedMu.Lock()
	defer t.feedMu.Unlock()

	// Debug: capture each source segment fed to THIS transcoder instance, in
	// feed order, so a wedging input sequence can be replayed into a
	// streamTranscoder in a test. No-op unless --segment-debug-dir is set. The
	// per-instance sequence number is baked into the name (not just the dump's
	// async wall-clock stamp) so a burst of sub-second feeds still sorts in feed
	// order. Numbering is per-instance, so the run that actually wedges (before
	// a reset rebuilds a fresh transcoder) is a self-contained 0..N sequence.
	seq := t.fedSeq
	t.fedSeq++
	t.mm.cli.DumpDebugSegment(t.ctx, fmt.Sprintf("transcoder-feed-%s-%05d.m4s", t.target, seq), bytes.NewReader(src))

	t.mu.Lock()
	switch {
	case t.closed:
		t.mu.Unlock()
		return fmt.Errorf("stream transcoder closed")
	case t.err != nil:
		err := t.err
		t.mu.Unlock()
		return err
	}
	first := !t.started
	t.started = true
	t.mu.Unlock()

	// Synthesize and write the init (ftyp+moov) once, then the segment bytes —
	// the same init-then-blind-concat the RTMP push feeder uses.
	if first {
		var init bytes.Buffer
		if err := muxl.RunMuxlWrapInit(t.ctx, bytes.NewReader(src), &init); err != nil {
			return fmt.Errorf("synthesize transcoder init: %w", err)
		}
		if _, err := t.feedW.Write(init.Bytes()); err != nil {
			return err
		}
	}
	// Enqueue the job before feeding the bytes so the consumer has the source
	// ready by the time its (lagging) transcoded twin emerges.
	select {
	case t.jobs <- transcodeJob{src: src, token: token}:
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
	if _, err := t.feedW.Write(src); err != nil {
		return err
	}
	return nil
}

// Close flushes the tail (EOS → final segment) and tears the pipeline down.
func (t *streamTranscoder) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	started := t.started
	t.mu.Unlock()

	if t.reaper != nil {
		t.reaper.Stop()
	}
	t.feedW.Close() // EOS to the appsrc → flush the final fragment(s)
	if started {
		select {
		case <-t.done:
		case <-time.After(15 * time.Second):
			t.cancel() // pipeline wedged on shutdown — force it down
			<-t.done
		}
	} else {
		t.cancel()
		<-t.done
	}
	t.mu.Lock()
	err := t.err
	t.mu.Unlock()
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
		return err
	}
	return nil
}

// run builds the continuous pipeline, drives muxl's segmenter over its output,
// and finishes each emitted transcoded segment against its source.
func (t *streamTranscoder) run(feedR *io.PipeReader) error {
	ctx := t.ctx
	pipeline, err := buildAudioTranscodePipeline(t.target)
	if err != nil {
		return err
	}
	defer func() {
		if e := pipeline.SetState(gst.StateNull); e != nil {
			log.Error(ctx, "stream transcode: set null", "error", e)
		}
	}()

	srcEle, err := pipeline.GetElementByName("src")
	if err != nil {
		return err
	}
	app.SrcFromElement(srcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, feedR),
	})

	// appsink output → io.Pipe → muxl streaming segmenter.
	muxR, muxW := io.Pipe()
	sink, err := pipeline.GetElementByName("sink")
	if err != nil {
		return err
	}
	app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, muxW),
	})

	eventCh := make(chan *muxl.MuxlEvent, 16)
	segErr := make(chan error, 1)
	go func() {
		segErr <- muxl.RunMuxlSegmenterEvents(ctx, muxR, eventCh)
		close(eventCh)
	}()

	busErr := make(chan error, 1)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		muxW.CloseWithError(err) // EOF/err to the muxl reader so it flushes + ends
		busErr <- err
	}()

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("stream transcode: set playing: %w", err)
	}

	// Consume per-GoP transcoded segments, pairing each 1:1 (in order) with the
	// source segment that produced it.
	var catalog *muxl.MuxlCatalog
	for ev := range eventCh {
		switch ev.Type {
		case "init":
			if ev.Catalog != nil {
				catalog = ev.Catalog
			}
			continue
		case "segment", "signed-segment":
		default:
			continue
		}

		audioTID, ok := audioTrackID(catalog)
		if !ok {
			log.Error(ctx, "stream transcode: no audio track in transcoded catalog")
			continue
		}
		transAudio := ev.Tracks[strconv.FormatUint(uint64(audioTID), 10)]

		var job transcodeJob
		select {
		case job = <-t.jobs:
		case <-ctx.Done():
			return ctx.Err()
		}
		if len(transAudio) == 0 {
			log.Error(ctx, "stream transcode: emitted segment missing audio track", "track", audioTID)
			continue
		}

		// Pass the WHOLE transcoded segment (video + audio), not the audio alone:
		// finishTranscodedSegment re-canonicalizes it to relabel the audio track,
		// and canonicalize needs the video keyframe as its cut clock to emit one
		// segment per GoP. Audio by itself has no keyframes to anchor on.
		transSeg := concatTracksByID(ev.Tracks)
		completed, err := t.mm.finishTranscodedSegment(ctx, job.src, transSeg, audioTID, t.cert, t.keyPEM)
		if err != nil {
			log.Error(ctx, "stream transcode: finish segment failed", "error", err)
			continue
		}
		t.onComplete(job.token, completed)
	}

	// eventCh closed: prefer a real pipeline/segmenter error over a clean EOF.
	if e := <-busErr; e != nil && !errors.Is(e, context.Canceled) {
		return e
	}
	if e := <-segErr; e != nil && !errors.Is(e, io.EOF) && !errors.Is(e, io.ErrClosedPipe) {
		return e
	}
	return nil
}

// finishTranscodedSegment assembles one completed dual-codec segment: relabel
// the freshly-segmented transcoded audio (the audio track transTID within
// transSeg) to a free track id so it won't collide with the source tracks, sign
// it as a c2pa.transcoded derivative of the source segment's audio (node
// identity), and append it to the source segment. The relabel is a lossless
// re-container (no re-encode), so the continuous encoder's gaplessness is
// preserved.
//
// transSeg is the FULL emitted transcoded segment (video + transcoded audio),
// not the audio track alone. The relabel goes through muxl's canonicalize,
// which re-segments its input; with audio only there are no keyframes to anchor
// on, so it falls back to a ~1s cadence and splits each ~2s GoP's audio in two —
// and only the first survived extraction, silently halving the transcoded track
// (every output segment carried ~1s of audio against ~2s of video, audible as
// the audio cutting out for the back half of each segment). Carrying the
// passed-through video as the cut clock makes canonicalize emit one segment per
// GoP, exactly like the source track.
func (mm *MediaManager) finishTranscodedSegment(ctx context.Context, srcSeg, transSeg []byte, transTID uint32, cert, keyPEM []byte) ([]byte, error) {
	events, err := unwrapMuxlEvents(ctx, srcSeg)
	if err != nil {
		return nil, fmt.Errorf("unwrap source segment: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Audio == nil {
		return nil, fmt.Errorf("source segment has no audio track")
	}
	var srcAudioTID, maxTID uint32
	if cat.Video != nil {
		for _, v := range cat.Video.Renditions {
			maxTID = maxU32(maxTID, v.TrackID())
		}
	}
	for _, a := range cat.Audio.Renditions {
		maxTID = maxU32(maxTID, a.TrackID())
		srcAudioTID = a.TrackID()
	}
	sourceAudio := tracks[strconv.FormatUint(uint64(srcAudioTID), 10)]
	if len(sourceAudio) == 0 {
		return nil, fmt.Errorf("source audio track %d missing", srcAudioTID)
	}
	freeTID := maxTID + 1

	// Relabel transTID → freeTID: wrap the full segment to fMP4, canonicalize
	// with the remap (the video keyframe anchors one segment per GoP), extract
	// the relabeled audio track. Pure re-container; no re-encode. Concatenate the
	// track across every emitted segment so nothing is dropped if canonicalize
	// ever splits the GoP (see catalogAndTracks / concatTrackAcrossSegments).
	var fmp4 bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(transSeg), "fmp4", &fmp4); err != nil {
		return nil, fmt.Errorf("wrap transcoded segment: %w", err)
	}
	canon, err := muxl.RunMuxlCanonicalize(ctx, fmp4.Bytes(), map[uint32]uint32{transTID: freeTID})
	if err != nil {
		return nil, fmt.Errorf("relabel transcoded audio: %w", err)
	}
	canonEvents, err := unwrapMuxlEvents(ctx, canon)
	if err != nil {
		return nil, fmt.Errorf("unwrap relabeled audio: %w", err)
	}
	output := concatTrackAcrossSegments(canonEvents, freeTID)
	if len(output) == 0 {
		return nil, fmt.Errorf("relabeled audio track %d missing", freeTID)
	}

	signed, err := muxl.RunMuxlSignTranscode(ctx, muxl.TranscodeInput{
		Output:   output,
		Source:   sourceAudio,
		CertPEM:  cert,
		KeyPEM:   keyPEM,
		Manifest: transcodeManifest(mm.cli.BroadcasterDID()),
	})
	if err != nil {
		return nil, fmt.Errorf("sign transcoded track: %w", err)
	}

	completed := make([]byte, 0, len(srcSeg)+len(signed))
	completed = append(completed, srcSeg...)
	completed = append(completed, signed...)
	return completed, nil
}

// audioTrackID returns the (single) audio rendition's track id in a catalog.
func audioTrackID(cat *muxl.MuxlCatalog) (uint32, bool) {
	if cat == nil || cat.Audio == nil {
		return 0, false
	}
	for _, a := range cat.Audio.Renditions {
		return a.TrackID(), true
	}
	return 0, false
}
