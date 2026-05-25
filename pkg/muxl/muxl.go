package muxl

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing/fstest"

	_ "embed"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/log"
)

var muxlTracer = otel.Tracer("muxl")

var moduleCounter atomic.Uint64

// MuxlEvent represents one event from the muxl segmenter/concatenator
// stdout stream. The wire format is `crate::cbor::CborEvent` on the muxl
// Rust side — a `type`-tagged union with "init" and "segment" variants.
// muxl-sign's `sign-segment` subcommand emits the same shape with a
// "signed-segment" type tag (the per-track bytes carry a leading
// c2pa-uuid box); we decode it into this same struct.
// Fields not relevant to a given Type are left zero.
type MuxlEvent struct {
	Type   string `cbor:"type"`
	Number uint32 `cbor:"number,omitempty"`

	// --- Init event ---
	// Data is the full ftyp+moov for ALL tracks combined — same bytes
	// the live ingest path writes straight to its output.
	Data []byte `cbor:"data,omitempty"`
	// Catalog describes per-track codec/dimensions/timescale/etc.
	Catalog *MuxlCatalog `cbor:"catalog,omitempty"`
	// TrackInits is per-track standalone ftyp+moov bytes (one per track)
	// keyed by stringified track ID. Used to construct HLS per-track
	// init segments addressable by their own BDASL CID.
	TrackInits map[string][]byte `cbor:"track_inits,omitempty"`

	// --- Segment event ---
	// Tracks is per-track moof+mdat bytes keyed by stringified track ID.
	Tracks map[string][]byte `cbor:"tracks,omitempty"`
	// Durations is per-track segment duration in timescale ticks.
	Durations map[string]uint64 `cbor:"durations,omitempty"`
	// SampleCounts is per-track sample (frame) count for this segment.
	SampleCounts map[string]uint32 `cbor:"sample_counts,omitempty"`
	// BodySize is the total bytes this segment contributes to the
	// concatenated output (sum of len(Tracks[*])).
	BodySize uint64 `cbor:"body_size,omitempty"`
	// DurationUs is the segment's playable wall duration in microseconds.
	DurationUs uint64 `cbor:"duration_us,omitempty"`
}

// MuxlCatalog mirrors `crate::catalog::Catalog` from muxl (the Rust
// authoritative type). We mirror only the fields the metafile pipeline
// needs; unknown fields are ignored on decode.
type MuxlCatalog struct {
	Video *MuxlCatalogVideo `cbor:"video,omitempty"`
	Audio *MuxlCatalogAudio `cbor:"audio,omitempty"`
}

type MuxlCatalogVideo struct {
	Renditions map[string]MuxlVideoConfig `cbor:"renditions"`
}

type MuxlCatalogAudio struct {
	Renditions map[string]MuxlAudioConfig `cbor:"renditions"`
}

type MuxlVideoConfig struct {
	Codec       string        `cbor:"codec"`
	Container   MuxlContainer `cbor:"container"`
	CodedWidth  uint32        `cbor:"codedWidth"`
	CodedHeight uint32        `cbor:"codedHeight"`
}

type MuxlAudioConfig struct {
	Codec            string        `cbor:"codec"`
	Container        MuxlContainer `cbor:"container"`
	SampleRate       uint32        `cbor:"sampleRate"`
	NumberOfChannels uint32        `cbor:"numberOfChannels"`
}

// MuxlContainer is the tagged-union container descriptor. Kind is
// either "cmaf" (the streamplace default) or "legacy". For "cmaf"
// the timescale/trackId fields are populated; for "legacy" they are
// zero.
type MuxlContainer struct {
	Kind      string `cbor:"kind"`
	Timescale uint32 `cbor:"timescale,omitempty"`
	TrackID   uint32 `cbor:"trackId,omitempty"`
}

// TrackID returns the configured CMAF track ID, or 0 for legacy.
func (c MuxlVideoConfig) TrackID() uint32 { return c.Container.TrackID }

// TrackID returns the configured CMAF track ID, or 0 for legacy.
func (c MuxlAudioConfig) TrackID() uint32 { return c.Container.TrackID }

// Timescale returns the media timescale (ticks per second), or 0 for legacy.
func (c MuxlVideoConfig) Timescale() uint32 { return c.Container.Timescale }

// Timescale returns the media timescale (ticks per second), or 0 for legacy.
func (c MuxlAudioConfig) Timescale() uint32 { return c.Container.Timescale }

// muxl.wasm is built from rust/muxl-wasm via `make muxl-wasm`. It bundles
// the full muxl-sign CLI — both unsigned subcommands (segment, concat,
// catalog, fmp4, mp4, hls) and the signing ones (sign-per-track,
// sign-segment) — so this package only needs one wasm artifact.
//
//go:embed muxl.wasm
var wasmBytes []byte

var wasmRuntime wazero.Runtime

// Compile the wasm module exactly once and reuse the result; instantiation
// is cheap, compilation is not. The signer runs for the length of a stream
// and the wrap/verify helpers run per segment, so the difference adds up fast.
var (
	compileOnce sync.Once
	compiled    wazero.CompiledModule
	compileErr  error
)

// signerRegistry holds the per-instance host-sign closure used by
// muxl-sign's `--host-sign` mode. The wasm import looks the closure up by
// the instance name (which we make unique per call via moduleCounter), so
// concurrent signs don't collide. RunMuxlSignSegment registers a closure on
// entry and deletes it on return.
var signerRegistry sync.Map // string → func([]byte) ([]byte, error)

// hostSignErr is the sentinel u32 muxl-sign's host_sign import returns to
// signal "the host couldn't sign this" — anything other than a real
// signature length.
const hostSignErr = ^uint32(0)

// memoryConfig holds the per-instance wasm linear memory tuning. wazero's
// default allocator reallocs+memcpys on every memory.grow page, so a
// module that ends up at 50MB allocates ~25GB of cumulative slices on its
// way there. The custom allocator below pre-allocates the backing buffer
// and grows geometrically, so a typical segment never reallocs at all.
//
// initial is the upfront capacity of the backing []byte; max is a hard
// ceiling — Reallocate returns nil past it, which surfaces to the wasm
// module as a memory.grow failure. Defaults are conservative; Configure
// overrides them from CLI flags.
var (
	memoryConfigMu     sync.RWMutex
	memoryInitialBytes uint64 = 50 * 1024 * 1024
	memoryMaxBytes     uint64 = 1024 * 1024 * 1024
)

// Configure sets the wasm linear memory tuning used by all subsequent
// RunMuxl* calls. Safe to call concurrently with in-flight calls (they'll
// keep their existing allocator) but typically called once at startup.
func Configure(initialBytes, maxBytes uint64) {
	memoryConfigMu.Lock()
	defer memoryConfigMu.Unlock()
	memoryInitialBytes = initialBytes
	memoryMaxBytes = maxBytes
}

func memoryConfigSnapshot() (initial, max uint64) {
	memoryConfigMu.RLock()
	defer memoryConfigMu.RUnlock()
	return memoryInitialBytes, memoryMaxBytes
}

// muxlAllocator implements experimental.MemoryAllocator. Stateless apart
// from the configured ceilings and the per-call ctx/instance used to log
// cap-exceeded events; each Allocate call produces a fresh
// muxlLinearMemory.
type muxlAllocator struct {
	ctx          context.Context
	instanceName string
	initialBytes uint64
	maxBytes     uint64
}

func (a *muxlAllocator) Allocate(capHint, wasmMax uint64) experimental.LinearMemory {
	effectiveMax := a.maxBytes
	if wasmMax > 0 && wasmMax < effectiveMax {
		effectiveMax = wasmMax
	}
	initial := a.initialBytes
	if initial < capHint {
		initial = capHint
	}
	if initial > effectiveMax {
		initial = effectiveMax
	}
	return &muxlLinearMemory{
		ctx:          a.ctx,
		instanceName: a.instanceName,
		buf:          make([]byte, 0, initial),
		max:          effectiveMax,
	}
}

// muxlLinearMemory is the per-instance backing buffer. Reallocate keeps
// the same slice (no copy) when the new size fits in the existing
// capacity; otherwise it doubles capacity (geometric growth) up to max,
// or returns nil if the requested size exceeds max.
type muxlLinearMemory struct {
	ctx          context.Context
	instanceName string
	buf          []byte
	max          uint64
}

func (m *muxlLinearMemory) Reallocate(size uint64) []byte {
	if size > m.max {
		log.Error(m.ctx, "muxl memory cap exceeded",
			"instance", m.instanceName,
			"requested_bytes", size,
			"max_bytes", m.max,
		)
		return nil
	}
	if size <= uint64(cap(m.buf)) {
		m.buf = m.buf[:size]
		return m.buf
	}
	newCap := uint64(cap(m.buf)) * 2
	if newCap < size {
		newCap = size
	}
	if newCap > m.max {
		newCap = m.max
	}
	newBuf := make([]byte, size, newCap)
	copy(newBuf, m.buf)
	m.buf = newBuf
	return m.buf
}

func (m *muxlLinearMemory) Free() {
	m.buf = nil
}

func init() {
	ctx := context.Background()
	wasmRuntime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, wasmRuntime)

	// Register the `muxl` host module that muxl.wasm imports. The
	// imports are declared unconditionally on the wasm side (they're part
	// of the binary's import table whether or not the corresponding wasm
	// path is exercised) so the host module must always exist. PEM-mode
	// sign invocations simply never call into host_sign; in-wasm SHA-256
	// invocations never call into host_sha256.
	_, err := wasmRuntime.NewHostModuleBuilder("muxl").
		NewFunctionBuilder().
		WithFunc(hostSign).
		Export("host_sign").
		NewFunctionBuilder().
		WithFunc(hostSha256).
		Export("host_sha256").
		Instantiate(ctx)
	if err != nil {
		panic(fmt.Errorf("registering muxl host module: %w", err))
	}
}

// hostSha256 is the trampoline behind muxl-sign's
// `muxl.host_sha256` import. Reads the input from wasm linear
// memory at (dataPtr, dataLen), hashes it with native Go's SHA-256
// (which has hardware-accelerated paths via the `crypto/sha256` package
// on amd64/arm64), and writes the 32-byte digest back at outPtr.
//
// Used today by the bench-sha256 subcommand to size the upper bound on
// what host SHA-256 saves vs in-wasm sha2; if the win is real and the
// patch story for c2pa-rs's sha2 dep gets settled, this becomes the
// hot-path implementation for all hashing too.
func hostSha256(ctx context.Context, mod api.Module, dataPtr, dataLen, outPtr uint32) {
	_, span := muxlTracer.Start(ctx, "muxl.hostSha256", trace.WithAttributes(
		attribute.String("instance", mod.Name()),
		attribute.Int64("data_len", int64(dataLen)),
	))
	defer span.End()

	data, ok := mod.Memory().Read(dataPtr, dataLen)
	if !ok {
		log.Error(ctx, "host_sha256: bad data pointer/length", "instance", mod.Name(), "ptr", dataPtr, "len", dataLen)
		span.SetAttributes(attribute.String("error", "bad data pointer"))
		return
	}
	span.AddEvent("read input bytes")
	sum := sha256.Sum256(data)
	span.AddEvent("hashed")
	if !mod.Memory().Write(outPtr, sum[:]) {
		log.Error(ctx, "host_sha256: bad output pointer", "instance", mod.Name(), "ptr", outPtr)
		span.SetAttributes(attribute.String("error", "bad output pointer"))
	}
}

// hostSign is the trampoline behind muxl-sign's `muxl.host_sign`
// import. It looks up the per-instance closure registered by
// RunMuxlSignSegment, hands it the bytes to sign, and writes the signature
// back into wasm memory. Returns the signature length on success or
// hostSignErr on any failure.
func hostSign(ctx context.Context, mod api.Module, dataPtr, dataLen, outPtr, outMax uint32) uint32 {
	ctx, span := muxlTracer.Start(ctx, "muxl.hostSign", trace.WithAttributes(
		attribute.String("instance", mod.Name()),
		attribute.Int64("data_len", int64(dataLen)),
	))
	defer span.End()

	v, ok := signerRegistry.Load(mod.Name())
	if !ok {
		log.Error(ctx, "host_sign called with no signer registered", "instance", mod.Name())
		span.SetAttributes(attribute.String("error", "no signer registered"))
		return hostSignErr
	}
	signFn := v.(func([]byte) ([]byte, error))
	span.AddEvent("registry lookup ok")

	data, ok := mod.Memory().Read(dataPtr, dataLen)
	if !ok {
		log.Error(ctx, "host_sign: bad data pointer/length", "instance", mod.Name(), "ptr", dataPtr, "len", dataLen)
		span.SetAttributes(attribute.String("error", "bad data pointer"))
		return hostSignErr
	}
	span.AddEvent("read input bytes")

	signCtx, signSpan := muxlTracer.Start(ctx, "muxl.hostSign.signFn")
	sig, err := signFn(data)
	signSpan.End()
	_ = signCtx
	if err != nil {
		log.Error(ctx, "host_sign: signer returned error", "instance", mod.Name(), "error", err)
		span.SetAttributes(attribute.String("error", err.Error()))
		return hostSignErr
	}
	span.SetAttributes(attribute.Int("sig_len", len(sig)))

	if uint32(len(sig)) > outMax {
		log.Error(ctx, "host_sign: signature too long for output buffer", "instance", mod.Name(), "len", len(sig), "max", outMax)
		span.SetAttributes(attribute.String("error", "signature too long"))
		return hostSignErr
	}
	if !mod.Memory().Write(outPtr, sig) {
		log.Error(ctx, "host_sign: bad output pointer", "instance", mod.Name(), "ptr", outPtr)
		span.SetAttributes(attribute.String("error", "bad output pointer"))
		return hostSignErr
	}
	span.AddEvent("wrote signature")
	return uint32(len(sig))
}

func getModule(ctx context.Context) (wazero.CompiledModule, error) {
	compileOnce.Do(func() {
		_, span := muxlTracer.Start(ctx, "muxl.CompileModule", trace.WithAttributes(
			attribute.Int("wasm_bytes", len(wasmBytes)),
		))
		compiled, compileErr = wasmRuntime.CompileModule(ctx, wasmBytes)
		span.End()
	})
	if compileErr != nil {
		return nil, fmt.Errorf("error compiling muxl wasm module: %w", compileErr)
	}
	return compiled, nil
}

// RunMuxlSegmenterEvents decodes the muxl segmenter's DRISL stream and
// delivers each *MuxlEvent (init + per-GoP segment, carrying the catalog,
// per-track init segments, durations, and sample counts) on eventCh. This is
// the unsigned counterpart of RunMuxlSignSegment, suitable for driving the
// live HLS writer (pkg/livehls) without a signing key. eventCh is NOT closed
// by this call; the caller closes it once the function returns.
func RunMuxlSegmenterEvents(ctx context.Context, input io.Reader, eventCh chan *MuxlEvent) error {
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "segment", "-", "--stdout"}, nil, false, input, nil, nil, nil, nil, eventCh)
}

// RunMuxlWrap synthesizes a presentation MP4 from a MUXL wrapper — a bare
// .m4s segment stream, a MUXL fMP4, or a flat MP4 — via muxl's `wrap`
// subcommand. format is "fmp4" (appendable: ftyp+moov(init) + verbatim
// segments) or "flat" (finalized faststart). The segment bytes, and any
// C2PA/S2PA signatures over them, pass through untouched; only the
// ftyp+moov header is synthesized from the segments' embedded catalogs.
//
// This is the inbound header-synthesis that makes a stored canonical .m4s
// understandable to gstreamer / transmux / players. Deterministic (fake
// clock — pure structural assembly, no signing).
func RunMuxlWrap(ctx context.Context, input io.Reader, format string, output io.Writer) error {
	if format == "" {
		format = "fmp4"
	}
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "wrap", "-", "-", "--format", format}, nil, false, input, output, nil, nil, nil, nil)
}

// RunMuxlWrapInit synthesizes only the per-stream init segment (ftyp+moov)
// from a MUXL wrapper's embedded catalogs — the HLS EXT-X-MAP target.
// Equivalent to `wrap --format fmp4 --init-only`.
func RunMuxlWrapInit(ctx context.Context, input io.Reader, output io.Writer) error {
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "wrap", "-", "-", "--format", "fmp4", "--init-only"}, nil, false, input, output, nil, nil, nil, nil)
}

// RunMuxlVerify validates the C2PA/S2PA signatures on a signed MUXL wrapper
// (bare .m4s stream, fMP4, or flat MP4) entirely inside the wasm sandbox via
// muxl-sign's `verify` subcommand, returning the manifest+cert+validation
// JSON document:
//
//	{"segments":[{"track_id":N,"manifest":{..},"cert":"<pem chain>",
//	              "validation_results":{..},"validation_state":".."}, ..]}
//
// one entry per canonical segment. This replaces the iroh-streamplace c2pa
// uniffi binding (get_manifest_and_cert) for segment validation: each
// segment verifies standalone as the "m4s" asset it was signed as, so no
// synthesized header is involved in the hash. Runs against the real clock so
// any cert-validity-window checks observe wall time.
func RunMuxlVerify(ctx context.Context, input io.Reader) (string, error) {
	mod, err := getModule(ctx)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := runMuxlWith(ctx, mod, []string{"muxl-wasm", "verify"}, nil, true, input, &out, nil, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String(), nil
}

// RunMuxlConcatenatorEvents concatenates MUXL-compatible fMP4 archives
// (init+segments) into a single fMP4 stream; if the init segment changes a
// new init is emitted. Bytes route to initCh/segCh, and each decoded
// *MuxlEvent is also sent on eventCh (if non-nil) so the caller can inspect
// per-segment metadata (durations, sample counts, per-track init segments)
// for sidecar metafile generation.
func RunMuxlConcatenatorEvents(ctx context.Context, input io.Reader, initCh chan []byte, segCh chan []byte, eventCh chan *MuxlEvent) error {
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	return runMuxlWith(ctx, mod, []string{"muxl-wasm", "concat"}, nil, false, input, nil, nil, initCh, segCh, eventCh)
}

// RunMuxlSignSegment streams an fMP4 input through muxl-sign's
// `sign-segment` subcommand: the muxl segmenter splits the input
// per-GoP and C2PA-signs each canonical segment in place, so the bytes
// routed to segCh are [c2pa-uuid][muxl-uuid][moof][mdat] per track. The
// init/segment/event channels behave exactly like
// RunMuxlConcatenatorEvents — the wire stream is the same DRISL event
// format, just with a "signed-segment" type tag in place of "segment".
//
// Signing needs the real wall clock (c2pa-rs checks cert validity at
// sign time and draws COSE nonces from real randomness), so unlike the
// plain segmenter this runs with realClock=true and its output is not
// byte-stable across runs. The segment bytes come from the input reader; the
// signing backend is selected like the rest of muxl-sign:
// exactly one of in.KeyPEM (in-wasm PEM signing) or in.Sign (host-callback,
// for hardware-backed keys via the wasm host_sign import) must be set.
func RunMuxlSignSegment(ctx context.Context, input io.Reader, in SignerInput, initCh chan []byte, segCh chan []byte, eventCh chan *MuxlEvent) error {
	hasKey := len(in.KeyPEM) > 0
	hasSign := in.Sign != nil
	if hasKey == hasSign {
		return fmt.Errorf("muxl: exactly one of SignerInput.KeyPEM or SignerInput.Sign must be set")
	}
	if in.Alg == "" {
		in.Alg = "es256k"
	}
	mod, err := getModule(ctx)
	if err != nil {
		return err
	}
	keysFS := fstest.MapFS{
		"cert.pem":     {Data: in.CertPEM},
		"track.json":   {Data: in.TrackManifest},
		"wrapper.json": {Data: in.WrapperManifest},
	}
	args := []string{
		"muxl-wasm", "sign-segment",
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
	return runMuxlWith(ctx, mod, args, fsCfg, true, input, nil, in.Sign, initCh, segCh, eventCh)
}

// SignerInput is the input bundle for RunMuxlSignSegment. Exactly one
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
	CertPEM         []byte
	KeyPEM          []byte
	Sign            func(data []byte) ([]byte, error)
	TrackManifest   []byte
	WrapperManifest []byte
	// Alg defaults to "es256k" when empty.
	Alg string
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
	// EventCh emits the full *MuxlEvent for every wasm event in
	// addition to the byte-level dispatch on InitCh/SegCh. Use this
	// when you need per-segment metadata (durations, sample counts,
	// per-track init segments) — for instance, building the HLS
	// metafile sidecar. Closed alongside InitCh/SegCh on shutdown.
	EventCh chan *MuxlEvent
	done    chan error
}

// NewConcatenator starts the WASM concat process in the background.
// Write full fMP4 archives via Write(), receive processed output on
// InitCh, SegCh, and EventCh.
//
// InitCh receives a new init segment only when the track configuration changes.
// SegCh receives raw segment data (moof+mdat) that can be concatenated after an init.
// EventCh receives the full *MuxlEvent — same data plus per-segment metadata
// (durations, sample counts, per-track init segments) needed by the HLS
// metafile builder. All three channels are closed when the concatenator
// finishes (after Close + WASM exit).
func NewConcatenator(ctx context.Context) *Concatenator {
	initCh := make(chan []byte, 1)
	segCh := make(chan []byte, 16)
	eventCh := make(chan *MuxlEvent, 16)
	stdinReader, stdinWriter := io.Pipe()
	done := make(chan error, 1)

	c := &Concatenator{
		stdinWriter: stdinWriter,
		InitCh:      initCh,
		SegCh:       segCh,
		EventCh:     eventCh,
		done:        done,
	}

	go func() {
		err := RunMuxlConcatenatorEvents(ctx, stdinReader, initCh, segCh, eventCh)
		close(initCh)
		close(segCh)
		close(eventCh)
		done <- err
	}()

	return c
}

// NewSigningSegmenter is the signing counterpart to NewConcatenator: it
// drives muxl-sign's `sign-segment` subcommand instead of `concat`, so
// the bytes emitted on SegCh are C2PA-signed canonical segments
// ([c2pa-uuid][muxl-uuid][moof][mdat] per track). The Concatenator
// plumbing (Write/Close + the three channels) is identical; only the
// underlying wasm subcommand differs. Feed full fMP4 archives via
// Write(); receive signed output on InitCh, SegCh, and EventCh.
func NewSigningSegmenter(ctx context.Context, in SignerInput) *Concatenator {
	initCh := make(chan []byte, 1)
	segCh := make(chan []byte, 16)
	eventCh := make(chan *MuxlEvent, 16)
	stdinReader, stdinWriter := io.Pipe()
	done := make(chan error, 1)

	c := &Concatenator{
		stdinWriter: stdinWriter,
		InitCh:      initCh,
		SegCh:       segCh,
		EventCh:     eventCh,
		done:        done,
	}

	go func() {
		err := RunMuxlSignSegment(ctx, stdinReader, in, initCh, segCh, eventCh)
		close(initCh)
		close(segCh)
		close(eventCh)
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
func runMuxlWith(ctx context.Context, mod wazero.CompiledModule, args []string, fsCfg wazero.FSConfig, realClock bool, input io.Reader, stdout io.Writer, signFn func([]byte) ([]byte, error), initCh chan []byte, segCh chan []byte, eventCh chan *MuxlEvent) error {
	instanceID := moduleCounter.Add(1)
	instanceName := fmt.Sprintf("muxl-%d", instanceID)

	ctx, span := muxlTracer.Start(ctx, "muxl.runMuxlWith", trace.WithAttributes(
		attribute.String("instance", instanceName),
		attribute.StringSlice("args", args),
		attribute.Bool("real_clock", realClock),
		attribute.Bool("has_input", input != nil),
		attribute.Bool("has_stdout", stdout != nil),
		attribute.Bool("has_sign_fn", signFn != nil),
		attribute.Bool("parse_events", initCh != nil && segCh != nil),
	))
	defer span.End()

	if signFn != nil {
		signerRegistry.Store(instanceName, signFn)
		defer signerRegistry.Delete(instanceName)
		span.AddEvent("registered host signer")
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
	parseEvents := initCh != nil || segCh != nil || eventCh != nil
	if parseEvents {
		stdoutReader, stdoutWriter = io.Pipe()
		cfg = cfg.WithStdout(stdoutWriter)
	} else if stdout != nil {
		cfg = cfg.WithStdout(stdout)
	}
	span.AddEvent("config built")

	initialBytes, maxBytes := memoryConfigSnapshot()
	allocator := &muxlAllocator{
		ctx:          ctx,
		instanceName: instanceName,
		initialBytes: initialBytes,
		maxBytes:     maxBytes,
	}

	errCh := make(chan error, 1)
	go func() {
		// Span covers the wasm's entire run from instantiation through
		// exit + cleanup. Host calls (host_sign, host_sha256) made
		// during execution will be children of this span via the ctx
		// wazero passes through.
		instCtx, instSpan := muxlTracer.Start(ctx, "muxl.wasm.InstantiateModule", trace.WithAttributes(
			attribute.String("instance", instanceName),
			attribute.Int64("memory_initial_bytes", int64(initialBytes)),
			attribute.Int64("memory_max_bytes", int64(maxBytes)),
		))
		instCtx = experimental.WithMemoryAllocator(instCtx, allocator)
		instance, err := wasmRuntime.InstantiateModule(instCtx, mod, cfg)
		instSpan.End()
		if err != nil {
			log.Error(ctx, "error instantiating module", "error", err)
		}
		// wazero leaves the module registered on clean exit; close to free
		// its WASM memory. Without this the signer leaks ~10MB per segment.
		if instance != nil {
			closeCtx, closeSpan := muxlTracer.Start(ctx, "muxl.wasm.Instance.Close", trace.WithAttributes(
				attribute.String("instance", instanceName),
			))
			closeErr := instance.Close(closeCtx)
			closeSpan.End()
			if closeErr != nil {
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
			_, copySpan := muxlTracer.Start(ctx, "muxl.wasm.stdinCopy", trace.WithAttributes(
				attribute.String("instance", instanceName),
			))
			n, err := io.Copy(stdinWriter, input)
			copySpan.SetAttributes(attribute.Int64("bytes_copied", n))
			copySpan.End()
			if err != nil && !errors.Is(err, io.ErrClosedPipe) {
				log.Error(ctx, "error copying input to stdin", "error", err)
			}
			stdinWriter.Close()
		}()
	}

	if parseEvents {
		_, parseSpan := muxlTracer.Start(ctx, "muxl.wasm.parseEvents")
		err := ParseMuxlEvents(ctx, stdoutReader, initCh, segCh, eventCh)
		parseSpan.End()
		if err != nil {
			return fmt.Errorf("parsing events: %w", err)
		}
	}

	span.AddEvent("waiting on wasm exit")
	if wasmErr := <-errCh; wasmErr != nil {
		return fmt.Errorf("wasm execution: %w", wasmErr)
	}
	span.AddEvent("wasm exited")
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
		// Note: no ctx threading — the sync hostSign trampoline already
		// holds a span open for "muxl.hostSign.signFn" that this work is
		// running under. Sub-spans here would only show up if the
		// closure was called directly from a Go context; not worth the
		// allocation for the common (host_sign-driven) path.
		digest := sha256Sum(data)
		sig, err := signer.Sign(rand.Reader, digest[:], cryptoSHA256)
		if err != nil {
			return nil, fmt.Errorf("muxl host sign: %w", err)
		}
		raw, ok := derECDSAToRaw(sig, byteLen)
		if !ok {
			return sig, nil
		}
		return raw, nil
	}
}

// ParseMuxlEvents decodes the muxl wasm's DRISL event stream and
// dispatches each event. Bytes are routed to initCh/segCh (legacy API,
// used by live ingest). If eventCh is non-nil it ALSO receives the
// full *MuxlEvent — used by callers that need the per-segment metadata
// (durations, sample counts, per-track init segments) to build a
// metafile sidecar. Any/all of the three channels may be nil.
//
// Per-segment per-track bytes are concatenated for segCh in sorted
// track-ID order (lex on the stringified ID) so the byte layout of the
// concatenated output is deterministic. Same ordering is what the
// metafile builder uses to compute per-track byte offsets.
func ParseMuxlEvents(ctx context.Context, r io.Reader, initCh chan []byte, segCh chan []byte, eventCh chan *MuxlEvent) error {
	decoder := drisl.NewDecoder(r)

	for {
		var ev MuxlEvent
		err := decoder.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("decode muxl event: %w", err)
		}
		switch ev.Type {
		case "init":
			if initCh != nil {
				select {
				case <-ctx.Done():
					return nil
				case initCh <- ev.Data:
				}
			}
		case "segment", "signed-segment":
			if segCh != nil {
				keys := make([]string, 0, len(ev.Tracks))
				for k := range ev.Tracks {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				combined := []byte{}
				for _, k := range keys {
					combined = append(combined, ev.Tracks[k]...)
				}
				select {
				case <-ctx.Done():
					return nil
				case segCh <- combined:
				}
			}
		default:
			return fmt.Errorf("unknown event type: %s", ev.Type)
		}
		if eventCh != nil {
			select {
			case <-ctx.Done():
				return nil
			case eventCh <- &ev:
			}
		}
	}

	return nil
}
