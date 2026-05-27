package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// MuxlSignSegmentElem builds the gstreamer bin that muxes the incoming
// video+audio into a fragmented MP4 stream, then drives muxl-sign's streaming
// per-segment signer over it. For each GoP it assembles the bare canonical
// .m4s — the per-track signed [c2pa-uuid][muxl-uuid][moof][mdat] runs
// concatenated in track-id order — and hands it to onSegment. That bare .m4s
// is exactly what gets stored, verified, and replicated; no flat MP4 is
// produced here. Presentation headers are synthesized downstream (ValidateMP4
// / playback) only when needed.
func MuxlSignSegmentElem(ctx context.Context, cli *config.CLI, ms MediaSigner, onSegment func(ctx context.Context, segment []byte) error) (*gst.Element, error) {
	ctx = log.WithLogValues(ctx, "func", "MuxlSignSegmentElem")
	bin := gst.NewBin("muxl-segment-bin")
	elem, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "fmp4mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return nil, err
	}
	if err := bin.Add(elem); err != nil {
		return nil, fmt.Errorf("failed to add mp4mux to bin: %w", err)
	}

	videoPad := elem.GetRequestPad("video_%u")
	if videoPad == nil {
		return nil, fmt.Errorf("failed to get video pad")
	}
	videoGhost := gst.NewGhostPad("video_0", videoPad)
	if videoGhost == nil {
		return nil, fmt.Errorf("failed to create video ghost pad")
	}
	audioPad := elem.GetRequestPad("audio_%u")
	if audioPad == nil {
		return nil, fmt.Errorf("failed to get audio pad")
	}
	audioGhost := gst.NewGhostPad("audio_0", audioPad)
	if audioGhost == nil {
		return nil, fmt.Errorf("failed to create audio ghost pad")
	}
	if ok := bin.AddPad(videoGhost.Pad); !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}
	if ok := bin.AddPad(audioGhost.Pad); !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": "muxl-appsink",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create appsink element: %w", err)
	}
	if err := bin.Add(appsink); err != nil {
		return nil, fmt.Errorf("failed to add appsink to bin: %w", err)
	}
	if err := elem.Link(appsink); err != nil {
		return nil, fmt.Errorf("failed to link mp4mux to appsink: %w", err)
	}

	r, w := io.Pipe()
	go func() {
		<-ctx.Done()
		r.Close()
	}()

	// Stream the fMP4 through the per-segment signer; each event carries one
	// GoP's per-track signed canonical segments.
	eventCh := make(chan *muxl.MuxlEvent, 16)
	go func() {
		err := ms.SignSegmentStream(ctx, r, eventCh)
		close(eventCh)
		if err != nil && ctx.Err() == nil {
			log.Error(ctx, "error running muxl sign-segment", "error", err)
		}
	}()
	go func() {
		for ev := range eventCh {
			if ev.Type != "signed-segment" {
				continue
			}
			segment := concatTracksSorted(ev.Tracks)
			cli.DumpDebugSegment(ctx, "muxl_signed_segment.m4s", bytes.NewReader(segment))
			if err := onSegment(ctx, segment); err != nil {
				log.Error(ctx, "error handling signed segment", "error", err)
			}
		}
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	return bin.Element, nil
}

// concatTracksSorted joins the per-track canonical segment bytes for one GoP
// in ascending track-id order — the canonical interleave a multi-track .m4s
// uses, which muxl's unwrap/verify/wrap all expect.
func concatTracksSorted(tracks map[string][]byte) []byte {
	keys := make([]string, 0, len(tracks))
	for k := range tracks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []byte
	for _, k := range keys {
		out = append(out, tracks[k]...)
	}
	return out
}
