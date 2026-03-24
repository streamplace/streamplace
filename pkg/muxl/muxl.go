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

//go:embed muxl.wasm
var wasmBytes []byte

var wasmRuntime wazero.Runtime

func init() {
	wasmRuntime = wazero.NewRuntime(context.Background())
	wasi_snapshot_preview1.MustInstantiate(context.Background(), wasmRuntime)
}

func getCompiledModule(ctx context.Context) (wazero.CompiledModule, error) {
	compiledModule, err := wasmRuntime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("error compiling module: %w", err)
	}
	return compiledModule, nil
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

// ConcatenatorEvents is like Concatenator but delivers structured MuxlEvents
// via a callback instead of flattened init/seg channels. This preserves the
// catalog, per-track data, durations, and sample counts from the CBOR stream.
type ConcatenatorEvents struct {
	stdinWriter *io.PipeWriter
	done        chan error
}

// NewConcatenatorEvents starts the WASM concat process in the background.
// Write full fMP4 archives via Write(). The onEvent callback is called for
// each structured MuxlEvent (init or segment) as they are produced.
// Call Close() when done writing; it blocks until the WASM process finishes.
func NewConcatenatorEvents(ctx context.Context, onEvent func(MuxlEvent) error) *ConcatenatorEvents {
	stdinReader, stdinWriter := io.Pipe()
	done := make(chan error, 1)

	c := &ConcatenatorEvents{
		stdinWriter: stdinWriter,
		done:        done,
	}

	go func() {
		err := runMuxlEvents(ctx, []string{"muxl", "concat"}, stdinReader, onEvent)
		done <- err
	}()

	return c
}

// Write feeds a full fMP4 archive (init+segments) to the concatenator.
func (c *ConcatenatorEvents) Write(data []byte) error {
	_, err := c.stdinWriter.Write(data)
	return err
}

// Close signals that no more data will be written. Blocks until the WASM
// process finishes and all events have been delivered.
func (c *ConcatenatorEvents) Close() error {
	c.stdinWriter.Close()
	return <-c.done
}

// runMuxlEvents is like runMuxl but uses ParseMuxlEventsFunc for structured output.
func runMuxlEvents(ctx context.Context, args []string, input io.Reader, onEvent func(MuxlEvent) error) error {
	compiledModule, err := getCompiledModule(ctx)
	if err != nil {
		return fmt.Errorf("error getting compiled module: %w", err)
	}

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	instanceID := moduleCounter.Add(1)
	config := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("muxl-%d", instanceID)).
		WithStdin(stdinReader).
		WithStdout(stdoutWriter).
		WithStderr(&logWriter{ctx: ctx, instanceID: instanceID}).
		WithArgs(args...)

	errCh := make(chan error, 1)
	go func() {
		_, err := wasmRuntime.InstantiateModule(ctx, compiledModule, config)
		if err != nil {
			log.Error(ctx, "error instantiating module", "error", err)
		}
		stdoutWriter.Close()
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(stdinWriter, input)
		if err != nil {
			log.Error(ctx, "error copying input to stdin", "error", err)
		}
		stdinWriter.Close()
	}()

	err = ParseMuxlEventsFunc(stdoutReader, onEvent)
	if err != nil {
		return fmt.Errorf("parsing events: %w", err)
	}

	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}

	return nil
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
	compiledModule, err := getCompiledModule(ctx)
	if err != nil {
		return fmt.Errorf("error getting compiled module: %w", err)
	}

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
		_, err := wasmRuntime.InstantiateModule(ctx, compiledModule, config)
		if err != nil {
			log.Error(ctx, "error instantiating module", "error", err)
		}
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
	err = ParseMuxlEvents(stdoutReader, initCh, segCh)
	if err != nil {
		return fmt.Errorf("parsing events: %w", err)
	}

	// Wait for WASM module to finish
	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}

	return nil
}

// ParseMuxlEvents decodes CBOR events and sends flattened init/segment bytes
// on the provided channels. This is the legacy interface used by the signing pipeline.
func ParseMuxlEvents(r io.Reader, initCh chan []byte, segCh chan []byte) error {
	return ParseMuxlEventsFunc(r, func(ev MuxlEvent) error {
		if ev.Type == "init" {
			initCh <- ev.Data
		} else if ev.Type == "segment" {
			combined := []byte{}
			for _, data := range ev.Tracks {
				combined = append(combined, data...)
			}
			segCh <- combined
		} else {
			return fmt.Errorf("unknown event type: %s", ev.Type)
		}
		return nil
	})
}

// ParseMuxlEventsFunc decodes CBOR events and calls the provided function for each one.
// This gives the caller access to the full structured MuxlEvent including catalog,
// per-track data, durations, and sample counts.
func ParseMuxlEventsFunc(r io.Reader, onEvent func(MuxlEvent) error) error {
	decoder := drisl.NewDecoder(r)
	for {
		var ev MuxlEvent
		err := decoder.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("decoding event: %w", err)
		}
		if err := onEvent(ev); err != nil {
			return err
		}
	}
	return nil
}

// RunMuxlSegmenterEvents runs the MUXL segmenter and calls onEvent for each
// structured CBOR event. Unlike RunMuxlSegmenter, this preserves the full
// MuxlEvent including catalog, per-track data, durations, and sample counts.
func RunMuxlSegmenterEvents(ctx context.Context, input io.Reader, onEvent func(MuxlEvent) error) error {
	return runMuxlEvents(ctx, []string{"muxl", "segment", "-", "--stdout"}, input, onEvent)
}
