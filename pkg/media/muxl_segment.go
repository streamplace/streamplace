package media

import (
	"bytes"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"

	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func MuxlSegmentElem(ctx context.Context, cli *config.CLI, streamer string, doH264Parse bool, cb func(ctx context.Context, buf []byte, now int64) error) (*gst.Element, error) {
	bin := gst.NewBin("muxl-segment-bin")
	elem, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "fmp4mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return nil, err
	}

	err = bin.Add(elem)
	if err != nil {
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

	ok := bin.AddPad(videoGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}

	ok = bin.AddPad(audioGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name": "muxl-appsink",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create appsink element: %w", err)
	}
	err = bin.Add(appsink)
	if err != nil {
		return nil, fmt.Errorf("failed to add appsink to bin: %w", err)
	}

	err = elem.Link(appsink)
	if err != nil {
		return nil, fmt.Errorf("failed to link mp4mux to appsink: %w", err)
	}

	initCh := make(chan []byte)
	segCh := make(chan []byte)
	r, w := io.Pipe()
	go func() {
		err := RunMuxlSegmenter(ctx, r, initCh, segCh)
		if err != nil {
			log.Error(ctx, "error running muxl segmenter", "error", err)
		}
	}()

	go func() {
		var initSeg []byte
		select {
		case <-ctx.Done():
			return
		case initSeg = <-initCh:
			log.Debug(ctx, "got init segment", "size", len(initSeg))
		}
		for {
			select {
			case <-ctx.Done():
				return
			case seg := <-segCh:
				log.Debug(ctx, "got segment", "size", len(seg))
				fullSeg := []byte{}
				fullSeg = append(fullSeg, initSeg...)
				fullSeg = append(fullSeg, seg...)
				cli.DumpDebugSegment(ctx, "muxl_segment_input.fmp4", bytes.NewReader(fullSeg))
				err := cb(ctx, fullSeg, time.Now().UnixMilli())
				if err != nil {
					log.Error(ctx, "error calling callback", "error", err)
				}
			}
		}
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
	})

	return bin.Element, nil
}

// Example: embedding muxl as a WASI module in Go via wazero.
//
// Pipes an fMP4 stream through the muxl WASI binary and reads back
// length-prefixed init segments and MUXL segments from stdout.
//
// Build the WASM binary first:
//   cargo build --target wasm32-wasip1 --release
//
// Then run this example:
//   go run ./examples/go-wasi /path/to/input.fmp4
//
// Or pipe from stdin:
//   cat input.fmp4 | go run ./examples/go-wasi -

// MuxlEvent represents an event from the muxl segmenter.
type MuxlEvent struct {
	Type   string // "INIT" or "SEGM"
	Number uint32 // segment number (only for SEGM)
	Data   []byte
}

//go:embed muxl.wasm
var wasmBytes []byte

var wasmRuntime wazero.Runtime

var compiledModule wazero.CompiledModule

func init() {
	var err error
	wasmRuntime = wazero.NewRuntime(context.Background())
	wasi_snapshot_preview1.MustInstantiate(context.Background(), wasmRuntime)
	// Compile the module
	compiledModule, err = wasmRuntime.CompileModule(context.Background(), wasmBytes)
	if err != nil {
		panic(err)
	}
}

// RunMuxlSegmenter runs the muxl WASI binary, piping the fMP4 input through
// stdin and parsing framed events from stdout.
//
// The WASM binary is loaded from the path in MUXL_WASM env var, or defaults
// to target/wasm32-wasip1/release/muxl.wasm.
func RunMuxlSegmenter(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	// Set up stdin/stdout pipes
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	config := wazero.NewModuleConfig().
		WithStdin(stdinReader).
		WithStdout(stdoutWriter).
		// WithStderr(os.Stderr).
		WithArgs("muxl", "segment", "-", "--stdout")

	// Run the module in a goroutine
	errCh := make(chan error, 1)
	go func() {
		_, err := wasmRuntime.InstantiateModule(ctx, compiledModule, config)
		stdoutWriter.Close()
		errCh <- err
	}()

	// Feed input to stdin in a goroutine
	go func() {
		_, err := io.Copy(stdinWriter, input)
		if err != nil {
			log.Error(ctx, "error copying input to stdin", "error", err)
		}
		stdinWriter.Close()
	}()

	// Parse framed events from stdout
	err := ParseMuxlEvents(stdoutReader, initCh, segCh)
	if err != nil {
		return fmt.Errorf("parsing events: %w", err)
	}

	// Wait for WASM module to finish
	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}

	return nil
}

// ParseMuxlEvents reads length-prefixed events from the muxl --stdout protocol.
//
// Protocol:
//
//	INIT: "INIT" (4 bytes) + uint32 data_length + data
//	SEGM: "SEGM" (4 bytes) + uint32 segment_number + uint32 data_length + data
func ParseMuxlEvents(r io.Reader, initCh chan []byte, segCh chan []byte) error {

	for {
		// Read 4-byte type tag
		var tag [4]byte
		_, err := io.ReadFull(r, tag[:])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}

		switch string(tag[:]) {
		case "INIT":
			var length uint32
			if err := binary.Read(r, binary.BigEndian, &length); err != nil {
				return fmt.Errorf("reading INIT length: %w", err)
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return fmt.Errorf("reading INIT data: %w", err)
			}
			initCh <- data

		case "SEGM":
			var number, length uint32
			if err := binary.Read(r, binary.BigEndian, &number); err != nil {
				return fmt.Errorf("reading SEGM number: %w", err)
			}
			if err := binary.Read(r, binary.BigEndian, &length); err != nil {
				return fmt.Errorf("reading SEGM length: %w", err)
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return fmt.Errorf("reading SEGM data: %w", err)
			}
			segCh <- data

		default:
			return fmt.Errorf("unknown tag: %q", tag)
		}
	}

	return nil
}
