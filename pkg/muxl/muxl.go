package muxl

import (
	"context"
	"errors"
	"fmt"
	"io"

	_ "embed"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"stream.place/streamplace/pkg/log"
)

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

func ParseMuxlEvents(r io.Reader, initCh chan []byte, segCh chan []byte) error {
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
			segCh <- ev.Data
		} else {
			return fmt.Errorf("unknown event type: %s", ev.Type)
		}
	}

	return nil
}
