// Package muxl wraps github.com/streamplace/muxl/go — the upstream MUXL/S2PA
// Go library, with its muxl-sign toolchain embedded as wasm — behind the
// package-level API streamplace has historically used. The wazero runtime,
// host imports (host_sign/host_sha256), and linear-memory tuning all live
// upstream now; this package is a thin adapter that builds one process-wide
// Engine and delegates to it. There is no longer a local wasm build step.
package muxl

import (
	"context"
	"io"
	"sync"

	upstream "github.com/streamplace/muxl/go"
)

// Re-exported upstream types under the names streamplace callers use. These are
// aliases, so values (events, catalogs, signer inputs) flow between this package
// and upstream with no conversion and the existing call sites are unchanged.
type (
	MuxlEvent        = upstream.Event
	MuxlCatalog      = upstream.Catalog
	MuxlCatalogVideo = upstream.CatalogVideo
	MuxlCatalogAudio = upstream.CatalogAudio
	MuxlVideoConfig  = upstream.VideoConfig
	MuxlAudioConfig  = upstream.AudioConfig
	MuxlContainer    = upstream.Container
	SignerInput      = upstream.SignerInput
	TranscodeInput   = upstream.TranscodeInput
)

// TranscodeIngredientLabel is the C2PA ingredient label SignTranscode assigns
// to the source segment; a TranscodeInput.Manifest references the source by
// listing it in an action's "org.cai.ingredientIds" (see the upstream doc).
const TranscodeIngredientLabel = upstream.TranscodeIngredientLabel

// SignerToCallback adapts a crypto.Signer into the host-sign callback that
// SignerInput.Sign / TranscodeInput.Sign expect (SHA-256 digest, ECDSA DER →
// fixed-width r‖s). See the upstream doc.
var SignerToCallback = upstream.SignerToCallback

// RawSignerToCallback adapts a raw-secp256k1 digest signer (e.g. an Ethereum
// keystore's SignHash) into the host-sign callback. See the upstream doc.
var RawSignerToCallback = upstream.RawSignerToCallback

// --- engine singleton -------------------------------------------------------

var (
	engineOnce sync.Once
	engine     *upstream.WASMEngine
	engineErr  error

	memMu      sync.Mutex
	memInitial = uint64(50 * 1024 * 1024)
	memMax     = uint64(1024 * 1024 * 1024)
)

// Configure sets the wasm linear-memory tuning (initial backing capacity and
// hard ceiling) applied when the Engine is first built. Call it before the
// first muxl operation — streamplace does, from its CLI bootstrap.
func Configure(initialBytes, maxBytes uint64) {
	memMu.Lock()
	defer memMu.Unlock()
	memInitial, memMax = initialBytes, maxBytes
}

// getEngine compiles the embedded muxl wasm once and returns the shared Engine.
// Compilation is the expensive step; each operation instantiates a fresh,
// isolated module internally, so the Engine is safe for concurrent use.
func getEngine() (*upstream.WASMEngine, error) {
	engineOnce.Do(func() {
		memMu.Lock()
		initial, max := memInitial, memMax
		memMu.Unlock()
		// Process-lifetime resource — build under Background so a cancelled
		// request context can't tear down the shared engine.
		engine, engineErr = upstream.NewWASM(context.Background(), upstream.WithMemory(initial, max))
	})
	return engine, engineErr
}

// --- delegating helpers -----------------------------------------------------

// RunMuxlWrap synthesizes a presentation MP4 from a MUXL wrapper. format is
// "fmp4" or "flat" (default fmp4).
func RunMuxlWrap(ctx context.Context, input io.Reader, format string, output io.Writer) error {
	eng, err := getEngine()
	if err != nil {
		return err
	}
	return eng.Wrap(ctx, input, format, output)
}

// RunMuxlWrapInit synthesizes only the per-stream init segment (ftyp+moov) —
// the HLS EXT-X-MAP target — from a MUXL wrapper's embedded catalogs.
func RunMuxlWrapInit(ctx context.Context, input io.Reader, output io.Writer) error {
	eng, err := getEngine()
	if err != nil {
		return err
	}
	return eng.WrapInit(ctx, input, output)
}

// RunMuxlVerify validates the C2PA/S2PA signatures on a signed MUXL wrapper and
// returns the per-segment manifest+cert+validation JSON document.
func RunMuxlVerify(ctx context.Context, input io.Reader) (string, error) {
	eng, err := getEngine()
	if err != nil {
		return "", err
	}
	return eng.Verify(ctx, input)
}

// RunMuxlUnwrapEvents re-derives the per-track event stream from a stored MUXL
// wrapper (bare .m4s, fMP4, or flat MP4), bytes and signatures verbatim.
func RunMuxlUnwrapEvents(ctx context.Context, input io.Reader, eventCh chan *MuxlEvent) error {
	eng, err := getEngine()
	if err != nil {
		return err
	}
	return eng.UnwrapEvents(ctx, input, eventCh)
}

// RunMuxlSegmenterEvents segments an fMP4 stream into per-GoP canonical MUXL
// events (unsigned).
func RunMuxlSegmenterEvents(ctx context.Context, input io.Reader, eventCh chan *MuxlEvent) error {
	eng, err := getEngine()
	if err != nil {
		return err
	}
	return eng.SegmentEvents(ctx, input, eventCh)
}

// RunMuxlSignSegment segments an fMP4 stream and S2PA-signs each canonical
// segment in place. Exactly one of in.KeyPEM or in.Sign must be set.
func RunMuxlSignSegment(ctx context.Context, input io.Reader, in SignerInput, initCh chan []byte, segCh chan []byte, eventCh chan *MuxlEvent) error {
	eng, err := getEngine()
	if err != nil {
		return err
	}
	return eng.SignSegment(ctx, input, in, initCh, segCh, eventCh)
}

// RunMuxlCanonicalize converts a flat or fragmented MP4 (e.g. a transcoder's
// output) into a canonical MUXL fMP4. trackRemap (may be nil) reassigns track
// IDs in the output — used to mint a transcoded rendition at a free id so it
// can join the source's tracks in one multi-track segment without colliding.
func RunMuxlCanonicalize(ctx context.Context, mp4 []byte, trackRemap map[uint32]uint32) ([]byte, error) {
	eng, err := getEngine()
	if err != nil {
		return nil, err
	}
	var opts []upstream.CanonicalizeOption
	if len(trackRemap) > 0 {
		opts = append(opts, upstream.WithTrackRemap(trackRemap))
	}
	return eng.Canonicalize(ctx, mp4, opts...)
}

// RunMuxlSignTranscode signs in.Output (an unsigned canonical MUXL segment —
// the transcoded result) as a standalone asset declaring in.Source (the
// canonical segment it was transcoded from) as a c2pa.transcoded parentOf
// ingredient. Returns the signed segment. Exactly one of in.KeyPEM or in.Sign
// must be set.
func RunMuxlSignTranscode(ctx context.Context, in TranscodeInput) ([]byte, error) {
	eng, err := getEngine()
	if err != nil {
		return nil, err
	}
	return eng.SignTranscode(ctx, in)
}

// --- push-style Concatenator ------------------------------------------------

// Concatenator is a push wrapper over the Engine: Write whole fMP4 archives,
// receive processed output on the channels, Close to finish. It mirrors the
// upstream Concatenator but adds an engine-construction error path on Close —
// the upstream value can't be built without a ready Engine, and callers expect
// construction never to fail (the error surfaces on Close).
//
//	cat := muxl.NewConcatenator(ctx)
//	go func() { cat.Write(fullFmp4Archive); cat.Close() }()
//	initSeg := <-cat.InitCh
//	for seg := range cat.SegCh { /* append to output */ }
type Concatenator struct {
	// InitCh receives an init segment only when the track configuration
	// changes. SegCh receives concatenable segment bodies (signed, for the
	// signing segmenter). EventCh receives the full *MuxlEvent with per-segment
	// metadata. All three are closed when processing finishes.
	InitCh  chan []byte
	SegCh   chan []byte
	EventCh chan *MuxlEvent

	write   func([]byte) error
	closeFn func() error
}

// Write feeds a full fMP4 archive (init+segments) to the pipeline.
func (c *Concatenator) Write(data []byte) error { return c.write(data) }

// Close signals end of input and waits for the pipeline to finish; the output
// channels are closed by the time it returns.
func (c *Concatenator) Close() error { return c.closeFn() }

// NewConcatenator drives Engine.ConcatEvents: dedup/concatenate MUXL fMP4
// archives into one stream (a new init is emitted only when the catalog
// changes).
func NewConcatenator(ctx context.Context) *Concatenator {
	eng, err := getEngine()
	if err != nil {
		return failedConcatenator(err)
	}
	return adopt(upstream.NewConcatenator(ctx, eng))
}

// NewSigningSegmenter drives Engine.SignSegment: each canonical segment is
// S2PA-signed in place, so SegCh carries [c2pa-uuid][muxl-uuid][moof][mdat] per
// track. Exactly one of in.KeyPEM or in.Sign must be set.
func NewSigningSegmenter(ctx context.Context, in SignerInput) *Concatenator {
	eng, err := getEngine()
	if err != nil {
		return failedConcatenator(err)
	}
	return adopt(upstream.NewSigningSegmenter(ctx, eng, in))
}

func adopt(c *upstream.Concatenator) *Concatenator {
	return &Concatenator{
		InitCh:  c.InitCh,
		SegCh:   c.SegCh,
		EventCh: c.EventCh,
		write:   c.Write,
		closeFn: c.Close,
	}
}

// failedConcatenator returns a Concatenator with closed output channels whose
// Write/Close report err, preserving the "engine error surfaces on Close"
// contract callers relied on.
func failedConcatenator(err error) *Concatenator {
	initCh, segCh, eventCh := make(chan []byte), make(chan []byte), make(chan *MuxlEvent)
	close(initCh)
	close(segCh)
	close(eventCh)
	return &Concatenator{
		InitCh:  initCh,
		SegCh:   segCh,
		EventCh: eventCh,
		write:   func([]byte) error { return err },
		closeFn: func() error { return err },
	}
}
