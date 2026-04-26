package muxl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	_ "embed"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"stream.place/streamplace/pkg/log"
)

var moduleCounter atomic.Uint64

// MuxlEvent represents an event from the muxl segmenter.
type MuxlEvent struct {
	Type   string // "INIT" or "SEGM"
	Number uint32 // segment number (only for SEGM)
	Tracks map[string][]byte
	Data   []byte
}

//go:embed muxl.wasm
var wasmBytes []byte

var (
	wasmRuntime    wazero.Runtime
	compiledModule wazero.CompiledModule
)

func init() {
	ctx := context.Background()
	wasmRuntime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, wasmRuntime)
	var err error
	compiledModule, err = wasmRuntime.CompileModule(ctx, wasmBytes)
	if err != nil {
		panic(fmt.Errorf("error compiling muxl wasm module: %w", err))
	}
}

// Segment arbitrary fMP4 input into MUXL-compatible init and segment chunks.
func RunMuxlSegmenter(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	return runMuxl(ctx, []string{"muxl", "segment", "-", "--stdout"}, input, initCh, segCh)
}

// Given a bunch of MUXL-compatible fMP4 archives containing init and segment chunks, concatenate them into a single fMP4 archive.
// If the init segment changes, you'll get a new init segment in the output.
func RunMuxlConcatenator(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	return runMuxl(ctx, []string{"muxl", "concat"}, input, initCh, segCh)
}

// Concatenator accepts full fMP4 archives (init+segments) and produces
// deduplicated output: init segments are emitted only when they change,
// and segment data is emitted without the init header, suitable for
// concatenation into a single fMP4 stream.
//
// Usage:
//
//	cat := muxl.NewConcatenator(ctx)
//	go func() { cat.Write(fullFmp4Archive); cat.Close() }()
//	initSeg := <-cat.InitCh
//	for seg := range cat.SegCh { /* append to output */ }
type Concatenator struct {
	stdinWriter *io.PipeWriter
	InitCh      chan []byte
	SegCh       chan []byte
	done        chan error
}

// NewConcatenator starts the WASM concat process in the background.
// Write full fMP4 archives via Write(), receive processed output on InitCh and SegCh.
// InitCh receives a new init segment only when the track configuration changes.
// SegCh receives raw segment data (moof+mdat) that can be concatenated after an init.
// Both channels are closed when the concatenator finishes (after Close + WASM exit).
func NewConcatenator(ctx context.Context) *Concatenator {
	initCh := make(chan []byte, 1)
	segCh := make(chan []byte, 16)
	stdinReader, stdinWriter := io.Pipe()
	done := make(chan error, 1)

	c := &Concatenator{
		stdinWriter: stdinWriter,
		InitCh:      initCh,
		SegCh:       segCh,
		done:        done,
	}

	go func() {
		err := RunMuxlConcatenator(ctx, stdinReader, initCh, segCh)
		close(initCh)
		close(segCh)
		done <- err
	}()

	return c
}

// Write feeds a full fMP4 archive (init+segments) to the concatenator.
func (c *Concatenator) Write(data []byte) error {
	_, err := c.stdinWriter.Write(data)
	return err
}

// Close signals that no more data will be written. The WASM process will
// finish processing and the output channels will be closed.
func (c *Concatenator) Close() error {
	c.stdinWriter.Close()
	return <-c.done
}

// logWriter adapts WASM stderr output to log.Debug calls, emitting one
// log message per line.
type logWriter struct {
	ctx        context.Context
	instanceID uint64
	buf        []byte
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := 0
		for i < len(w.buf) && w.buf[i] != '\n' {
			i++
		}
		if i >= len(w.buf) {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		log.Debug(w.ctx, "muxl wasm", "instance", w.instanceID, "msg", line)
	}
	return len(p), nil
}

func runMuxl(ctx context.Context, args []string, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	// Set up stdin/stdout pipes
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	instanceID := moduleCounter.Add(1)
	config := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("muxl-%d", instanceID)).
		WithStdin(stdinReader).
		WithStdout(stdoutWriter).
		WithStderr(&logWriter{ctx: ctx, instanceID: instanceID}).
		WithArgs(args...)

	// Run the module in a goroutine
	errCh := make(chan error, 1)
	go func() {
		mod, err := wasmRuntime.InstantiateModule(ctx, compiledModule, config)
		if err != nil {
			log.Error(ctx, "error instantiating module", "error", err)
		}
		// wazero leaves the module registered on clean exit; close to free its WASM memory.
		if mod != nil {
			if closeErr := mod.Close(ctx); closeErr != nil {
				log.Error(ctx, "error closing wasm module", "error", closeErr)
			}
		}
		stdoutWriter.Close()
		errCh <- err
	}()

	// Feed input to stdin in a goroutine
	go func() {
		_, err := io.Copy(stdinWriter, input)
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			log.Error(ctx, "error copying input to stdin", "error", err)
		}
		stdinWriter.Close()
	}()

	// Parse framed events from stdout
	if err := ParseMuxlEvents(ctx, stdoutReader, initCh, segCh); err != nil {
		return fmt.Errorf("parsing events: %w", err)
	}

	// Wait for WASM module to finish
	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}

	return nil
}

func ParseMuxlEvents(ctx context.Context, r io.Reader, initCh chan []byte, segCh chan []byte) error {
	decoder := drisl.NewDecoder(r)

	for {
		var ev MuxlEvent
		err := decoder.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if ev.Type == "init" {
			initCh <- ev.Data
		} else if ev.Type == "segment" {
			combined := []byte{}
			for _, data := range ev.Tracks {
				combined = append(combined, data...)
			}
			select {
			case <-ctx.Done():
				return nil
			case segCh <- combined:
			}
		} else {
			return fmt.Errorf("unknown event type: %s", ev.Type)
		}
	}

	return nil
}
