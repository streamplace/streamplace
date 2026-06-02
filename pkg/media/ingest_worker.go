package media

import (
	"context"
	"fmt"
	"io"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// IngestWorkerConfig is the startup handshake the main process hands an ingest
// worker over a dedicated pipe fd — kept off argv/env so key material never
// lands in a process listing.
//
// INTERIM key custody: per the locked design the worker signs everything, so it
// receives the streamer key directly. This is the deliberately-temporary
// approach; the detach/reattach work will revisit how a worker holds keys.
type IngestWorkerConfig struct {
	StreamerDID string `json:"streamer_did"`
	// KeyPEM is the streamer's ES256K signing key in PEM. The MKV/RTMP push path
	// always yields a software key, which is what muxl-sign wants here; it is
	// forwarded verbatim, no reconstruction.
	KeyPEM  []byte `json:"key_pem"`
	CertPEM []byte `json:"cert_pem"`
	// Manifest is the C2PA manifest JSON, built ONCE by main at stream start.
	// muxl-sign stamps each segment's signing time into it as it signs. NOTE:
	// static for the worker's lifetime — mid-stream manifest changes (e.g. a
	// pre-live → live transition) don't yet cross the boundary; that needs a
	// control channel and is tracked as future work.
	Manifest []byte `json:"manifest"`
}

// RunMKVIngestWorker is the body of the `ingest-worker` subcommand. It reads an
// MKV stream from stdin, runs the same demux + Opus re-encode + muxl-sign
// pipeline as the in-process MKVIngest, and emits each signed canonical .m4s
// segment to frames; the main process reads those frames and runs ValidateMP4
// over each, exactly as if onSegment had called it directly.
//
// It returns when the stream ends cleanly (EOS) or the pipeline errors. The
// caller frames End or Error accordingly. All segment frames are guaranteed
// flushed before it returns, so a trailing End can never race ahead of the last
// Segment.
func RunMKVIngestWorker(ctx context.Context, cfg IngestWorkerConfig, stdin io.Reader, frames *ingestframe.Writer) error {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The worker signs everything itself: forward the streamer key PEM + cert +
	// prebuilt manifest straight to muxl-sign. No MediaSigner / model / DB needed.
	signStream := func(ctx context.Context, input io.Reader, eventCh chan *muxl.MuxlEvent) error {
		fetchManifest := func() ([]byte, error) { return cfg.Manifest, nil }
		return muxl.RunMuxlSignSegment(ctx, input, muxl.SignerInput{
			CertPEM:           cfg.CertPEM,
			KeyPEM:            cfg.KeyPEM,
			TrackManifestFn:   fetchManifest,
			WrapperManifestFn: fetchManifest,
		}, nil, nil, eventCh)
	}

	onSegment := func(_ context.Context, segment []byte) error {
		return frames.Segment(segment)
	}

	signerElem, done, err := muxlSignSegmentElem(ctx, &config.CLI{}, signStream, onSegment)
	if err != nil {
		return fmt.Errorf("build signer element: %w", err)
	}
	pipeline, err := buildMKVIngestPipeline(ctx, stdin, signerElem)
	if err != nil {
		return fmt.Errorf("build pipeline: %w", err)
	}

	busErr := make(chan error, 1)
	go func() {
		busErr <- HandleBusMessages(ctx, pipeline)
	}()

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("set playing: %w", err)
	}
	defer func() {
		if err := pipeline.SetState(gst.StateNull); err != nil {
			log.Error(ctx, "ingest worker: set null", "error", err)
		}
	}()

	// Wait for the pipeline to finish (EOS or error), then drain the signer:
	// cancelling unblocks the signer's input pipe so it flushes the final GoP,
	// and <-done guarantees every segment frame is written before we return.
	pipeErr := <-busErr
	cancel()
	<-done
	return pipeErr
}
