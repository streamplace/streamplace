package muxl

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing/fstest"

	_ "embed"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
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

// muxl.wasm is built from rust/muxl-wasm via `make muxl-wasm`. It bundles
// the full muxl-sign CLI — both unsigned subcommands (segment, concat,
// catalog, fmp4, mp4, hls) and the signing ones (sign-per-track,
// sign-segment) — so this package only needs one wasm artifact.
//
//go:embed muxl.wasm
var wasmBytes []byte

var wasmRuntime wazero.Runtime

// Compile the wasm module exactly once and reuse the result; instantiation
// is cheap, compilation is not. RunMuxlSigner runs once per GoP so the
// difference adds up fast.
var (
	compileOnce sync.Once
	compiled    wazero.CompiledModule
	compileErr  error
)

// signerRegistry holds the per-instance host-sign closure used by
// muxl-sign's `--host-sign` mode. The wasm import looks the closure up by
// the instance name (which we make unique per call via moduleCounter), so
// concurrent signs don't collide. RunMuxlSigner registers a closure on
// entry and deletes it on return.
var signerRegistry sync.Map // string → func([]byte) ([]byte, error)

// hostSignErr is the sentinel u32 muxl-sign's host_sign import returns to
// signal "the host couldn't sign this" — anything other than a real
// signature length.
const hostSignErr = ^uint32(0)

func init() {
	ctx := context.Background()
	wasmRuntime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, wasmRuntime)

	// Register the `streamplace` host module that muxl.wasm imports. The
	// imports are declared unconditionally on the wasm side (they're part
	// of the binary's import table whether or not the corresponding wasm
	// path is exercised) so the host module must always exist. PEM-mode
	// sign invocations simply never call into host_sign; in-wasm SHA-256
	// invocations never call into host_sha256.
	_, err := wasmRuntime.NewHostModuleBuilder("streamplace").
		NewFunctionBuilder().
		WithFunc(hostSign).
		Export("host_sign").
		NewFunctionBuilder().
		WithFunc(hostSha256).
		Export("host_sha256").
		Instantiate(ctx)
	if err != nil {
		panic(fmt.Errorf("registering streamplace host module: %w", err))
	}
}

// hostSha256 is the trampoline behind muxl-sign's
// `streamplace.host_sha256` import. Reads the input from wasm linear
// memory at (dataPtr, dataLen), hashes it with native Go's SHA-256
// (which has hardware-accelerated paths via the `crypto/sha256` package
// on amd64/arm64), and writes the 32-byte digest back at outPtr.
//
// Used today by the bench-sha256 subcommand to size the upper bound on
// what host SHA-256 saves vs in-wasm sha2; if the win is real and the
// patch story for c2pa-rs's sha2 dep gets settled, this becomes the
// hot-path implementation for all hashing too.
func hostSha256(ctx context.Context, mod api.Module, dataPtr, dataLen, outPtr uint32) {
	data, ok := mod.Memory().Read(dataPtr, dataLen)
	if !ok {
		log.Error(ctx, "host_sha256: bad data pointer/length", "instance", mod.Name(), "ptr", dataPtr, "len", dataLen)
		return
	}
	sum := sha256.Sum256(data)
	if !mod.Memory().Write(outPtr, sum[:]) {
		log.Error(ctx, "host_sha256: bad output pointer", "instance", mod.Name(), "ptr", outPtr)
	}
}

// hostSign is the trampoline behind muxl-sign's `streamplace.host_sign`
// import. It looks up the per-instance closure registered by
// RunMuxlSigner, hands it the bytes to sign, and writes the signature
// back into wasm memory. Returns the signature length on success or
// hostSignErr on any failure.
func hostSign(ctx context.Context, mod api.Module, dataPtr, dataLen, outPtr, outMax uint32) uint32 {
	v, ok := signerRegistry.Load(mod.Name())
	if !ok {
		log.Error(ctx, "host_sign called with no signer registered", "instance", mod.Name())
		return hostSignErr
	}
	signFn := v.(func([]byte) ([]byte, error))

	data, ok := mod.Memory().Read(dataPtr, dataLen)
	if !ok {
		log.Error(ctx, "host_sign: bad data pointer/length", "instance", mod.Name(), "ptr", dataPtr, "len", dataLen)
		return hostSignErr
	}
	sig, err := signFn(data)
	if err != nil {
		log.Error(ctx, "host_sign: signer returned error", "instance", mod.Name(), "error", err)
		return hostSignErr
	}
	if uint32(len(sig)) > outMax {
		log.Error(ctx, "host_sign: signature too long for output buffer", "instance", mod.Name(), "len", len(sig), "max", outMax)
		return hostSignErr
	}
	if !mod.Memory().Write(outPtr, sig) {
		log.Error(ctx, "host_sign: bad output pointer", "instance", mod.Name(), "ptr", outPtr)
		return hostSignErr
	}
	return uint32(len(sig))
}

func getModule(ctx context.Context) (wazero.CompiledModule, error) {
	compileOnce.Do(func() {
		compiled, compileErr = wasmRuntime.CompileModule(ctx, wasmBytes)
	})
	if compileErr != nil {
		return nil, fmt.Errorf("error compiling muxl wasm module: %w", compileErr)
	}
	return compiled, nil
}

// Segment arbitrary fMP4 input into MUXL-compatible init and segment chunks.
func RunMuxlSegmenter(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "segment", "-", "--stdout"}, nil, false, input, nil, nil, initCh, segCh)
}

// Given a bunch of MUXL-compatible fMP4 archives containing init and segment chunks, concatenate them into a single fMP4 archive.
// If the init segment changes, you'll get a new init segment in the output.
func RunMuxlConcatenator(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte) error {
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "concat"}, nil, false, input, nil, nil, initCh, segCh)
}

// SignerInput is the per-call input bundle for RunMuxlSigner. Exactly one
// of KeyPEM or Sign must be set:
//
//   - KeyPEM: the streamer's PKCS#8-PEM private key bytes. Sent into the
//     wasm sandbox via a read-only FS mount; signing happens inside wasm
//     using c2pa-rs. Use this for software keys where the bytes are
//     readily available (e.g. atproto-derived stream keys).
//   - Sign: a host-side closure that takes pre-hashed-or-not data (per
//     c2pa's CallbackSigner contract: ECDSA receives the unhashed bytes
//     and the closure is expected to do SHA-256 + sign + raw r||s) and
//     returns the signature. Use this for hardware-backed signers
//     (PKCS#11, EIP-712) whose key bytes never leave the host. Powered
//     by the wasm `streamplace.host_sign` import — see hostSign.
//
// Cert chain is always PEM bytes, leaf first. Manifests are JSON bodies
// already substituted with per-segment values (timestamps etc.).
type SignerInput struct {
	Segment         []byte
	CertPEM         []byte
	KeyPEM          []byte
	Sign            func(data []byte) ([]byte, error)
	TrackManifest   []byte
	WrapperManifest []byte
	// Alg defaults to "es256k" when empty.
	Alg string
}

// RunMuxlSigner signs one segment's worth of MP4 bytes via the muxl-sign
// `sign-per-track` subcommand. Returns the wrapper-signed flat MP4 with
// per-track ingredient manifests embedded.
//
// All I/O is in-memory: the segment streams in on stdin, the signed output
// streams out on stdout, and cert/manifests (plus key.pem in PEM mode) are
// exposed via a read-only fs.FS mount at /keys. No host filesystem hits —
// important on Windows where %TEMP% lives on NTFS and per-call
// open/write/close + Defender scans show up under load.
func RunMuxlSigner(ctx context.Context, in SignerInput) ([]byte, error) {
	hasKey := len(in.KeyPEM) > 0
	hasSign := in.Sign != nil
	if hasKey == hasSign {
		return nil, fmt.Errorf("muxl: exactly one of SignerInput.KeyPEM or SignerInput.Sign must be set")
	}
	if in.Alg == "" {
		in.Alg = "es256k"
	}
	mod, err := getModule(ctx)
	if err != nil {
		return nil, err
	}

	keysFS := fstest.MapFS{
		"cert.pem":     {Data: in.CertPEM},
		"track.json":   {Data: in.TrackManifest},
		"wrapper.json": {Data: in.WrapperManifest},
	}
	args := []string{
		"muxl-wasm", "sign-per-track",
		"--input", "-",
		"--output", "-",
		"--cert", "/keys/cert.pem",
		"--alg", in.Alg,
		"--track-manifest", "/keys/track.json",
		"--wrapper-manifest", "/keys/wrapper.json",
	}
	if hasKey {
		keysFS["key.pem"] = &fstest.MapFile{Data: in.KeyPEM}
		args = append(args, "--key", "/keys/key.pem")
	} else {
		args = append(args, "--host-sign")
	}
	fsCfg := wazero.NewFSConfig().WithFSMount(keysFS, "/keys")
	var output bytes.Buffer
	if err := runMuxlWith(ctx, mod, args, fsCfg, true, bytes.NewReader(in.Segment), &output, in.Sign, nil, nil); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
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

// logWriter adapts WASM stderr output to log calls, one message per line.
// Lines tagged "Error:" are surfaced at error level so signing failures
// aren't lost in debug noise.
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
		if strings.HasPrefix(line, "Error:") || strings.HasPrefix(line, "thread '") {
			log.Error(w.ctx, "muxl wasm error", "instance", w.instanceID, "msg", line)
		} else {
			log.Debug(w.ctx, "muxl wasm", "instance", w.instanceID, "msg", line)
		}
	}
	return len(p), nil
}

// stderrWriter lets tests override how wasm stderr is captured. Production
// uses logWriter. Tests can swap in os.Stderr to surface clap/c2pa errors
// directly (slog's default config drops debug-level logs in tests).
var stderrWriter func(ctx context.Context, instanceID uint64) io.Writer = func(ctx context.Context, instanceID uint64) io.Writer {
	return &logWriter{ctx: ctx, instanceID: instanceID}
}

// runMuxlWith instantiates a precompiled wasm module with the given args and
// optional FS mount. If initCh+segCh are non-nil, stdout is parsed as DRISL
// events and routed to those channels; otherwise if stdout is non-nil the
// module's stdout writes go straight there. If input is non-nil, it's piped
// to the module's stdin. If signFn is non-nil it's registered against the
// instance's name for the duration of the call so that wasm calls into
// `streamplace.host_sign` route to it. realClock=true exposes the host's
// wall clock and real randomness — c2pa-rs needs both for cert validity
// checks and COSE sign nonces. The segmenter intentionally runs against
// wazero's fake clock so its output stays byte-stable across runs.
func runMuxlWith(ctx context.Context, mod wazero.CompiledModule, args []string, fsCfg wazero.FSConfig, realClock bool, input io.Reader, stdout io.Writer, signFn func([]byte) ([]byte, error), initCh chan []byte, segCh chan []byte) error {
	instanceID := moduleCounter.Add(1)
	instanceName := fmt.Sprintf("muxl-%d", instanceID)
	if signFn != nil {
		signerRegistry.Store(instanceName, signFn)
		defer signerRegistry.Delete(instanceName)
	}
	cfg := wazero.NewModuleConfig().
		WithName(instanceName).
		WithStderr(stderrWriter(ctx, instanceID)).
		WithArgs(args...)
	if realClock {
		cfg = cfg.
			WithSysWalltime().
			WithSysNanotime().
			WithSysNanosleep().
			WithRandSource(rand.Reader)
	}
	if fsCfg != nil {
		cfg = cfg.WithFSConfig(fsCfg)
	}

	var stdinReader *io.PipeReader
	var stdinWriter *io.PipeWriter
	if input != nil {
		stdinReader, stdinWriter = io.Pipe()
		cfg = cfg.WithStdin(stdinReader)
	}

	var stdoutReader *io.PipeReader
	var stdoutWriter *io.PipeWriter
	parseEvents := initCh != nil && segCh != nil
	if parseEvents {
		stdoutReader, stdoutWriter = io.Pipe()
		cfg = cfg.WithStdout(stdoutWriter)
	} else if stdout != nil {
		cfg = cfg.WithStdout(stdout)
	}

	errCh := make(chan error, 1)
	go func() {
		instance, err := wasmRuntime.InstantiateModule(ctx, mod, cfg)
		if err != nil {
			log.Error(ctx, "error instantiating module", "error", err)
		}
		// wazero leaves the module registered on clean exit; close to free
		// its WASM memory. Without this RunMuxlSigner leaks ~10MB per GoP.
		if instance != nil {
			if closeErr := instance.Close(ctx); closeErr != nil {
				log.Error(ctx, "error closing wasm module", "error", closeErr)
			}
		}
		if stdoutWriter != nil {
			stdoutWriter.Close()
		}
		errCh <- err
	}()

	if input != nil {
		go func() {
			_, err := io.Copy(stdinWriter, input)
			if err != nil && !errors.Is(err, io.ErrClosedPipe) {
				log.Error(ctx, "error copying input to stdin", "error", err)
			}
			stdinWriter.Close()
		}()
	}

	if parseEvents {
		if err := ParseMuxlEvents(ctx, stdoutReader, initCh, segCh); err != nil {
			return fmt.Errorf("parsing events: %w", err)
		}
	}

	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}
	return nil
}

// SignerToCallback wraps a crypto.Signer for use as SignerInput.Sign.
//
// Hashes data with SHA-256 (matching c2pa's CallbackSigner contract for
// SHA-256-family algs — ECDSA P-256/secp256k1, RSA PS256), calls the
// signer, and converts ECDSA DER output to raw r||s. byteLen is the
// curve's coordinate byte size: 32 for ES256/ES256K, 48 for ES384, 66
// for ES512. For RSA-PSS algs the byteLen argument is ignored and the
// raw signer output is passed through.
//
// The crypto.Signer can be a software ecdsa.PrivateKey, a PKCS#11
// hardware signer, an EIP-712 wallet wrapper, etc. — anything implementing
// the standard interface.
func SignerToCallback(signer cryptoSigner, byteLen int) func([]byte) ([]byte, error) {
	return func(data []byte) ([]byte, error) {
		digest := sha256Sum(data)
		sig, err := signer.Sign(rand.Reader, digest[:], cryptoSHA256)
		if err != nil {
			return nil, fmt.Errorf("muxl host sign: %w", err)
		}
		// ECDSA returns DER; c2pa-rs callbacks must produce raw r||s.
		// Detect by trying to parse — RSA-PSS returns a signature that
		// won't parse as the SEQUENCE { r INTEGER, s INTEGER } shape, so
		// we pass it through.
		raw, ok := derECDSAToRaw(sig, byteLen)
		if !ok {
			return sig, nil
		}
		return raw, nil
	}
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
