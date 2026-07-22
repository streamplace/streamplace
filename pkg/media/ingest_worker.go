package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httputil"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// manifestHolder holds the worker's current C2PA manifest. It starts as the
// manifest main built at spawn (cfg.Manifest) and is swapped when main pushes an
// updated one over the socket (an ingestframe.Manifest control frame). The
// signer reads the latest per GoP, so a pre-live → live transition reaches the
// worker mid-stream — mirroring the in-process signer's fresh-per-GoP manifest,
// which the worker otherwise can't do (it has no model). Concurrent-safe: the
// signer reads while the socket goroutine writes.
type manifestHolder struct {
	mu sync.RWMutex
	b  []byte
}

func newManifestHolder(initial []byte) *manifestHolder { return &manifestHolder{b: initial} }

func (h *manifestHolder) get() []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.b
}

func (h *manifestHolder) set(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.b = b
}

// IngestWorkerConfig is the startup handshake the main process hands an ingest
// worker over a dedicated pipe fd — kept off argv/env so key material never
// lands in a process listing.
//
// INTERIM key custody: per the locked design the worker signs everything, so it
// receives the streamer key directly. This is the deliberately-temporary
// approach; the detach/reattach work will revisit how a worker holds keys.
type IngestWorkerConfig struct {
	StreamerDID string `json:"streamer_did"`
	// KeyPEM is the streamer's ES256K signing key in PEM. The Mist-pull/RTMP path
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

	// InputFD, when > 0, is the fd main passed the ingest CONNECTION on (fd-passing
	// the accepted, authed push). The worker reads media from it directly instead
	// of stdin, so main is out of the media path and the worker keeps ingesting
	// across a main restart. 0 → read media from stdin.
	InputFD int `json:"input_fd,omitempty"`

	// Prebuf is the HTTP body bytes main had already read past the request headers
	// when it hijacked the push connection — prepended to the fd stream so none are
	// lost. Chunked says the body uses chunked transfer-encoding, so the worker
	// de-chunks the (prebuf+fd) stream to recover the raw media. Both are unset for
	// stdin / raw fd input.
	Prebuf  []byte `json:"prebuf,omitempty"`
	Chunked bool   `json:"chunked,omitempty"`

	// Record, when true, makes the worker write a debug recording of this session
	// (the fMP4 ingest body, or the WHIP session) under
	// DataDir/debug-recordings/<did>/. main evaluates the per-stream DebugRecording
	// setting (which needs the DB) and the worker carries it out — so debug
	// recording keeps working on the isolated paths without main being in the data
	// path, and a recording even survives a main restart. DataDir is set (only when
	// Record) to the node data dir the worker writes recordings under.
	Record  bool   `json:"record,omitempty"`
	DataDir string `json:"data_dir,omitempty"`

	// Transport selects the worker's ingest source: "" / "mp4" reads fragmented
	// MP4 media (stdin or InputFD); "whip" makes the worker own the WebRTC
	// PeerConnection, built from OfferSDP — no media fd to pass.
	Transport string `json:"transport,omitempty"`
	// OfferSDP is the WHIP client's SDP offer (transport "whip"). The worker
	// generates the answer and emits it as the first frame (ingestframe.Answer)
	// so main can return it to the client before consuming segments.
	OfferSDP string `json:"offer_sdp,omitempty"`
}

// IngestTransportWHIP is the cfg.Transport value selecting the WHIP worker.
const IngestTransportWHIP = "whip"

// WorkerInput reconstructs the raw media stream the gst pipeline reads from the
// fd-passed push connection: prepend any bytes main already read past the headers
// (Prebuf), then de-chunk if the push used chunked transfer-encoding. For stdin
// or a raw fd (no prebuf, not chunked) it returns raw unchanged.
func WorkerInput(cfg IngestWorkerConfig, raw io.Reader) io.Reader {
	r := raw
	if len(cfg.Prebuf) > 0 {
		r = io.MultiReader(bytes.NewReader(cfg.Prebuf), r)
	}
	if cfg.Chunked {
		r = httputil.NewChunkedReader(r)
	}
	return r
}

// workerSignStream returns the streaming muxl signer a worker uses: it forwards
// the streamer key PEM + cert straight to muxl-sign, no MediaSigner / model / DB
// needed. The manifest is read FRESH per GoP from the holder, so a manifest main
// pushes mid-stream (e.g. pre-live → live) takes effect on the next GoP — the
// same fresh-per-GoP shape as the in-process signer. Shared by the MP4 and WHIP
// workers.
func workerSignStream(cfg IngestWorkerConfig, getManifest func() []byte) SignSegmentStreamFunc {
	return func(ctx context.Context, input io.Reader, eventCh chan *muxl.MuxlEvent) error {
		fetchManifest := func() ([]byte, error) { return getManifest(), nil }
		return muxl.RunMuxlSignSegment(ctx, input, muxl.SignerInput{
			CertPEM:           cfg.CertPEM,
			KeyPEM:            cfg.KeyPEM,
			TrackManifestFn:   fetchManifest,
			WrapperManifestFn: fetchManifest,
		}, nil, nil, eventCh)
	}
}

// workerSegmentSink returns the onSegment handler a worker hands to
// muxlSignSegmentElem, plus a flush to call once the signer has drained. With a
// node transcode key it completes each single-codec source segment to dual-codec
// via an in-process transcoder (its completion callback frames the finished
// segment); flush Closes that transcoder so its ~1-GoP tail is framed before the
// worker exits. The transcoder runs on a non-cancellable context so draining the
// signer can't kill it early. One process == one session, so the per-DID
// transcoder-reuse hazard can't arise. Shared by the MP4 and WHIP workers.
func (mm *MediaManager) workerSegmentSink(ctx context.Context, cfg IngestWorkerConfig, frames FrameWriter) (onSegment func(context.Context, []byte) error, flush func()) {
	var transcoder *streamTranscoder
	onSegment = func(_ context.Context, segment []byte) error {
		if len(cfg.NodeKeyPEM) == 0 {
			return frames.Segment(segment) // no node signer → single-codec
		}
		if transcoder == nil {
			// WithoutCancel for the same reason as the transcoder below: this
			// runs from the signer's post-cancel drain, and a muxl unwrap
			// started with a cancelled ctx deadlocks its wasm mid-stream
			// instead of returning.
			target, need := mm.audioCompletionTarget(context.WithoutCancel(ctx), segment)
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
	flush = func() {
		if transcoder != nil {
			if cerr := transcoder.Close(); cerr != nil {
				log.Error(ctx, "ingest worker: transcoder close", "error", cerr)
			}
		}
	}
	return onSegment, flush
}

// RunMP4IngestWorker is the body of the `ingest-worker` subcommand. It reads a
// fragmented-MP4 stream from stdin, runs the same demux + Opus re-encode + muxl-sign
// pipeline as the in-process MP4Ingest, and emits each signed canonical .m4s
// segment to frames; the main process reads those frames and runs ValidateMP4
// over each, exactly as if onSegment had called it directly.
//
// It returns when the stream ends cleanly (EOS) or the pipeline errors. The
// caller frames End or Error accordingly. All segment frames are guaranteed
// flushed before it returns, so a trailing End can never race ahead of the last
// Segment.
func RunMP4IngestWorker(ctx context.Context, cfg IngestWorkerConfig, stdin io.Reader, frames FrameWriter, getManifest func() []byte) error {
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Self-watchdog: if the pipeline wedges and stops emitting frames, tear it
	// down so the worker exits and the fault stays contained (the only wedge
	// containment on the detached path — main can't kill a detached worker).
	wd := newWorkerWatchdog(ctx, ingestWorkerWatchdog, cancel, cfg.StreamerDID)
	defer wd.stop()
	frames = wd.wrap(frames)

	// Minimal manager: the broadcaster identity the transcode completion
	// (finishTranscodedSegment) stamps into the node-signed AAC track, plus the
	// data dir for an optional debug recording.
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: cfg.BroadcasterHost, DataDir: cfg.DataDir}}
	onSegment, flush := mm.workerSegmentSink(ctx, cfg, frames)

	// Debug recording: tee the ingest media to a file before it reaches gst. main
	// decided this (cfg.Record) and handed us DataDir; recording here keeps main
	// out of the data path and lets the recording survive a main restart.
	media := stdin
	if cfg.Record {
		log.Log(ctx, "recording ingest media to file", "streamer", cfg.StreamerDID)
		pr, pw := io.Pipe()
		media = io.TeeReader(stdin, pw)
		go func() {
			if derr := mm.dumpToFile(ctx, pr, cfg.StreamerDID, ".rtmp.mp4"); derr != nil {
				log.Error(ctx, "ingest worker: dump recording to file", "error", derr)
			}
		}()
	}

	signerElem, done, err := muxlSignSegmentElem(ctx, mm.cli, workerSignStream(cfg, getManifest), onSegment)
	if err != nil {
		return fmt.Errorf("build signer element: %w", err)
	}
	pipeline, err := buildMP4IngestPipeline(ctx, media, signerElem)
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

	// Pipeline done (EOS/error) → drain the signer (cancel flushes the final GoP;
	// <-done means every source segment has been fed) → flush the transcoder tail
	// so the last dual-codec completions are framed before we return.
	pipeErr := <-busErr
	cancel()
	<-done
	flush()
	return pipeErr
}
