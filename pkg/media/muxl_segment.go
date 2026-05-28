package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// muxlInputDumpDirEnv names a directory to duplicate each ingest's MUXL fMP4
// input — the raw gst mp4mux output that feeds the per-segment signer — to a
// file. Off unless set. Purely diagnostic: lets us inspect the gstreamer
// segmentation offline (e.g. feed it straight to `muxl segment`) to tell a muxl
// segmentation bug from an upstream webrtc-ingest keyframe drop.
const muxlInputDumpDirEnv = "SP_MUXL_INPUT_DEBUG_DIR"

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

	// Optional debug: duplicate the gst mp4mux output — muxl's exact fMP4 input
	// — to a file so the raw gstreamer segmentation can be inspected offline.
	// Best-effort: a dump-file error never disturbs the real signing pipe (its
	// Write result is preserved verbatim).
	var sampleW io.Writer = w
	if dir := os.Getenv(muxlInputDumpDirEnv); dir != "" {
		if dump, derr := openMuxlInputDump(dir, ms.Streamer()); derr != nil {
			log.Error(ctx, "muxl input dump: open failed", "dir", dir, "error", derr)
		} else {
			log.Log(ctx, "muxl input dump: capturing gst fMP4 input", "path", dump.Name())
			go func() {
				<-ctx.Done()
				dump.Close()
			}()
			sampleW = &teeWriter{primary: w, dup: dump, onErr: func(e error) {
				log.Error(ctx, "muxl input dump: write failed, stopping capture", "error", e)
			}}
		}
	}

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
		NewSampleFunc: WriterNewSample(ctx, sampleW),
	})

	return bin.Element, nil
}

// openMuxlInputDump creates the per-session fMP4 capture file under dir (named
// by sign time + streamer DID). The file is a complete fMP4 stream (ftyp+moov
// then moof/mdat per GoP) — replayable straight through `muxl segment`.
func openMuxlInputDump(dir, streamer string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s-muxl-input.fmp4",
		aqtime.FromTime(time.Now()).FileSafeString(),
		strings.ReplaceAll(streamer, ":", "-"))
	return os.Create(filepath.Join(dir, name))
}

// teeWriter mirrors writes to dup (best-effort) while returning primary's
// result verbatim, so the duplicate never alters the real pipeline's timing or
// errors. On the first dup error it reports once and stops mirroring. Not
// safe for concurrent use; the gst appsink calls NewSample serially.
type teeWriter struct {
	primary io.Writer
	dup     io.Writer
	onErr   func(error)
}

func (t *teeWriter) Write(p []byte) (int, error) {
	n, err := t.primary.Write(p)
	if t.dup != nil {
		if _, derr := t.dup.Write(p); derr != nil {
			if t.onErr != nil {
				t.onErr(derr)
			}
			t.dup = nil
		}
	}
	return n, err
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
