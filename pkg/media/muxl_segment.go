package media

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// SignSegmentStreamFunc drives muxl-sign's streaming per-segment signer over an
// fMP4 input, emitting one signed-segment event per GoP on eventCh. It is the
// only thing muxlSignSegmentElem needs from a signer, so the isolated ingest
// worker can supply a key-PEM-backed closure without a full MediaSigner (and
// without the model/DB a MediaSignerLocal carries).
type SignSegmentStreamFunc func(ctx context.Context, input io.Reader, eventCh chan *muxl.MuxlEvent) error

// MuxlSignSegmentElem builds the gstreamer bin that muxes the incoming
// video+audio into a fragmented MP4 stream, then drives muxl-sign's streaming
// per-segment signer over it. For each GoP it assembles the bare canonical
// .m4s — the per-track signed [c2pa-uuid][muxl-uuid][moof][mdat] runs
// concatenated in track-id order — and hands it to onSegment. That bare .m4s
// is exactly what gets stored, verified, and replicated; no flat MP4 is
// produced here. Presentation headers are synthesized downstream (ValidateMP4
// / playback) only when needed.
func MuxlSignSegmentElem(ctx context.Context, cli *config.CLI, ms MediaSigner, onSegment func(ctx context.Context, segment []byte) error) (*gst.Element, error) {
	elem, _, err := muxlSignSegmentElem(ctx, cli, ms.SignSegmentStream, onSegment)
	return elem, err
}

// muxlSignSegmentElem is MuxlSignSegmentElem's core, parameterized by the raw
// sign-stream function and additionally returning a done channel that closes
// once every signed segment has been drained to onSegment (the signer goroutine
// has finished and the event loop has emptied). The isolated ingest worker waits
// on it to guarantee all segment frames are flushed before it signals a clean
// end-of-stream.
func muxlSignSegmentElem(ctx context.Context, cli *config.CLI, signStream SignSegmentStreamFunc, onSegment func(ctx context.Context, segment []byte) error) (*gst.Element, <-chan struct{}, error) {
	ctx = log.WithLogValues(ctx, "func", "MuxlSignSegmentElem")
	bin := gst.NewBin("muxl-segment-bin")
	elem, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "fmp4mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := bin.Add(elem); err != nil {
		return nil, nil, fmt.Errorf("failed to add mp4mux to bin: %w", err)
	}

	videoPad := elem.GetRequestPad("video_%u")
	if videoPad == nil {
		return nil, nil, fmt.Errorf("failed to get video pad")
	}
	videoGhost := gst.NewGhostPad("video_0", videoPad)
	if videoGhost == nil {
		return nil, nil, fmt.Errorf("failed to create video ghost pad")
	}
	audioPad := elem.GetRequestPad("audio_%u")
	if audioPad == nil {
		return nil, nil, fmt.Errorf("failed to get audio pad")
	}
	audioGhost := gst.NewGhostPad("audio_0", audioPad)
	if audioGhost == nil {
		return nil, nil, fmt.Errorf("failed to create audio ghost pad")
	}
	if ok := bin.AddPad(videoGhost.Pad); !ok {
		return nil, nil, fmt.Errorf("failed to add video ghost pad to bin")
	}
	if ok := bin.AddPad(audioGhost.Pad); !ok {
		return nil, nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	// sync=false: this sink feeds the signer, not a display — render as fast as
	// upstream produces. The default (sync=true) made the appsink wait on the
	// pipeline clock per buffer, pacing the whole ingest graph at realtime:
	// harmless for a live source arriving at 1x, but it throttled tests/replays
	// and kept the graph's queues near-full for no benefit. Every other appsink
	// in the tree already sets this.
	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": "muxl-appsink",
		"sync": false,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create appsink element: %w", err)
	}
	if err := bin.Add(appsink); err != nil {
		return nil, nil, fmt.Errorf("failed to add appsink to bin: %w", err)
	}
	if err := elem.Link(appsink); err != nil {
		return nil, nil, fmt.Errorf("failed to link mp4mux to appsink: %w", err)
	}

	r, w := io.Pipe()
	go func() {
		<-ctx.Done()
		r.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Non-cancellable: cancelling ctx is the FLUSH signal, not an abort — see
		// signSegments.
		if err := signSegments(context.WithoutCancel(ctx), cli, signStream, r, onSegment); err != nil && ctx.Err() == nil {
			log.Error(ctx, "error running muxl sign-segment", "error", err)
		}
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	return bin.Element, done, nil
}

// signSegments streams an fMP4 input through the per-segment signer and hands
// each GoP's bare canonical .m4s to onSegment, returning once all of them have
// been handed over.
//
// ctx MUST NOT be cancellable; callers stop the signer by closing input
// instead. Closing input is the FLUSH signal, not an abort: the signer sees
// EOF, signs the final GoP, and exits cleanly. If a cancelled ctx reached
// muxl's event parser instead, the parser would abandon the stream mid-write
// and the signer wasm would deadlock against the unread stdout pipe, so this
// would never return. That was rare while ingest was clock-paced (everything
// had drained by EOS); at full speed EOS+cancel land while GoPs are still in
// flight, and the abort path lost every time.
func signSegments(ctx context.Context, cli *config.CLI, signStream SignSegmentStreamFunc, input io.Reader, onSegment func(ctx context.Context, segment []byte) error) error {
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := signStream(ctx, input, eventCh)
		close(eventCh)
		errCh <- err
	}()
	for ev := range eventCh {
		if ev.Type != "signed-segment" {
			continue
		}
		segment := concatTracksByID(ev.Tracks)
		cli.DumpDebugSegment(ctx, "muxl_signed_segment.m4s", bytes.NewReader(segment))
		if err := onSegment(ctx, segment); err != nil {
			log.Error(ctx, "error handling signed segment", "error", err)
		}
	}
	return <-errCh
}

// signFMP4Direct signs an fMP4 stream that needs no remuxing straight through
// the per-segment signer, with no GStreamer graph in front of it. That fits a
// client that already muxes every track the way a segment carries it — the
// Streamplace OBS plugin, whose encoders flag only IDR frames as keyframes —
// and it keeps every track the client sends, where the demux pipeline keeps
// one video and one audio track. Cancelling ctx flushes the final GoP; it
// returns once every segment has been handed to onSegment.
func signFMP4Direct(ctx context.Context, cli *config.CLI, signStream SignSegmentStreamFunc, input io.Reader, onSegment func(ctx context.Context, segment []byte) error) error {
	r, w := io.Pipe()
	// Closing r on return also releases the copy below if the signer stopped
	// reading early.
	defer r.Close()
	go func() {
		_, err := io.Copy(w, input)
		w.CloseWithError(err)
	}()
	stop := context.AfterFunc(ctx, func() { r.Close() })
	defer stop()
	err := signSegments(context.WithoutCancel(ctx), cli, signStream, r, onSegment)
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	return err
}
