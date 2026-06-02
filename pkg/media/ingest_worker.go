package media

import (
	"context"
	"fmt"
	"io"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
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

	// Node transcode signer + broadcaster identity. When set, the worker completes
	// each single-codec source segment to dual-codec (Opus+AAC) itself — the
	// transcode runs in this isolated process too — signing the added track under
	// the node identity. Empty → the worker emits single-codec source segments.
	NodeCertPEM     []byte `json:"node_cert_pem,omitempty"`
	NodeKeyPEM      []byte `json:"node_key_pem,omitempty"`
	BroadcasterHost string `json:"broadcaster_host,omitempty"`

	// SocketPath, when set, switches the worker to the detach/reattach transport:
	// it serves frames over this unix socket with buffered reconnect (survives a
	// main restart) instead of the fd-4 pipe. Empty → fd-4 pipe (Stage 1).
	SocketPath string `json:"socket_path,omitempty"`
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
func RunMKVIngestWorker(ctx context.Context, cfg IngestWorkerConfig, stdin io.Reader, frames FrameWriter) error {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Minimal manager: just the broadcaster identity the transcode completion
	// (finishTranscodedSegment) stamps into the node-signed AAC track.
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: cfg.BroadcasterHost}}

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

	// With a node transcode key, the worker completes each single-codec source
	// segment to dual-codec itself: feed the signed source segment into a
	// per-stream transcoder running in THIS process; its completion callback
	// frames the finished dual-codec segment. The transcoder runs on a
	// non-cancellable context so draining the signer (cancel, below) can't kill it
	// before its ~1-GoP tail is flushed by Close. One process == one session, so
	// the per-DID transcoder-reuse hazard simply can't arise here.
	var transcoder *streamTranscoder
	onSegment := func(_ context.Context, segment []byte) error {
		if len(cfg.NodeKeyPEM) == 0 {
			return frames.Segment(segment) // no node signer → single-codec
		}
		if transcoder == nil {
			target, need := mm.audioCompletionTarget(ctx, segment)
			if !need {
				return frames.Segment(segment) // already dual-codec / no audio track
			}
			transcoder = mm.newStreamTranscoder(context.WithoutCancel(ctx), target, cfg.NodeCertPEM, cfg.NodeKeyPEM,
				func(_ any, completed []byte) {
					if ferr := frames.Segment(completed); ferr != nil {
						log.Error(ctx, "ingest worker: frame completed segment", "error", ferr)
					}
				})
		}
		return transcoder.Feed(segment, nil)
	}

	signerElem, done, err := muxlSignSegmentElem(ctx, mm.cli, signStream, onSegment)
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
	// cancelling unblocks the signer's input pipe so it flushes the final GoP, and
	// <-done guarantees every source segment has been fed. Then flush the
	// transcoder's tail so the last dual-codec completions are framed before we
	// return (the caller's End can't race ahead of them).
	pipeErr := <-busErr
	cancel()
	<-done
	if transcoder != nil {
		if cerr := transcoder.Close(); cerr != nil {
			log.Error(ctx, "ingest worker: transcoder close", "error", cerr)
		}
	}
	return pipeErr
}
