package media

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/rtcrec"
)

// ServeWHIPIngestWorkerSocket is the WHIP counterpart of
// ServeMKVIngestWorkerSocket. Unlike MKV there's no socket/fd to pass in: the
// worker OWNS the PeerConnection, so it creates it from cfg.OfferSDP (binding its
// own UDP sockets), generates the SDP answer, and emits it as the FIRST frame on
// the unix socket — main reads that Answer frame and returns it to the WHIP
// client, then keeps reading the signed dual-codec segments. The worker is
// detached, so the WebRTC session (and the buffered segment stream) survive a
// main restart.
func ServeWHIPIngestWorkerSocket(ctx context.Context, cfg IngestWorkerConfig) error {
	if cfg.SocketPath == "" {
		return fmt.Errorf("ServeWHIPIngestWorkerSocket: empty socket path")
	}
	if cfg.OfferSDP == "" {
		return fmt.Errorf("ServeWHIPIngestWorkerSocket: empty offer")
	}
	gstinit.InitGST()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_ = os.Remove(cfg.SocketPath) // clear any stale socket from a prior worker
	ln, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.SocketPath, err)
	}
	defer func() {
		ln.Close()
		_ = os.Remove(cfg.SocketPath)
	}()

	srv := newFrameServer(workerFrameBuffer)
	go serveFrameSocket(ctx, ln, srv)

	// finish flushes the trailing End/Error, waits for main to drain the buffer
	// (incl. the Answer), then closes the connection for a clean EOF.
	finish := func(runErr error) error {
		if runErr != nil {
			_ = srv.Error(runErr.Error())
		} else {
			_ = srv.End()
		}
		srv.waitDrained(ctx, workerDrainGrace)
		srv.closeConn()
		return runErr
	}

	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: cfg.BroadcasterHost, DataDir: cfg.DataDir}}

	// The worker owns the PeerConnection (its own UDP sockets), built with the
	// same codec/interceptor setup as the in-process server. Debug recording is
	// decided by main (cfg.Record) and written by the worker under cfg.DataDir.
	api, webrtcConfig, err := newWebRTCAPI()
	if err != nil {
		return finish(fmt.Errorf("webrtc api: %w", err))
	}
	pionpc, err := api.NewPeerConnection(webrtcConfig)
	if err != nil {
		return finish(fmt.Errorf("peer connection: %w", err))
	}
	pc, err := rtcrec.NewRecordingPeerConnection(ctx, *mm.cli, cfg.StreamerDID, pionpc, cfg.Record)
	if err != nil {
		return finish(fmt.Errorf("peer connection wrapper: %w", err))
	}

	onSegment, flush := mm.workerSegmentSink(ctx, cfg, srv)
	signerElem, signerDone, err := muxlSignSegmentElem(ctx, mm.cli, workerSignStream(cfg), onSegment)
	if err != nil {
		return finish(fmt.Errorf("build signer element: %w", err))
	}

	offer := &webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: cfg.OfferSDP}
	streamDone := make(chan error, 1)
	answer, err := mm.webRTCIngestPipeline(ctx, cancel, offer, pc, signerElem, nil, streamDone)
	if err != nil {
		return finish(fmt.Errorf("webrtc ingest: %w", err))
	}

	// Hand the answer back to main FIRST; the segment frames stream behind it.
	if aerr := srv.Answer(answer.SDP); aerr != nil {
		log.Error(ctx, "whip worker: frame answer", "error", aerr)
	}

	// Streaming runs until the peer disconnects / errors; webRTCIngestPipeline
	// cancels ctx then, which drains the signer. Wait for that, flush the
	// transcoder tail, then finish.
	streamErr := <-streamDone
	cancel()
	<-signerDone
	flush()
	return finish(streamErr)
}
