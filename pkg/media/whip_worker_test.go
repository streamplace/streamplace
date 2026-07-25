package media

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// whipClientOffer builds a WHIP-style SDP offer (H264 video + Opus audio tracks),
// the way a real WHIP client does, returning the client PC, its tracks, and the
// offer.
func whipClientOffer(t *testing.T) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, *webrtc.TrackLocalStaticSample, webrtc.SessionDescription) {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	require.NoError(t, err)
	if _, err = pc.AddTrack(videoTrack); err != nil {
		t.Fatal(err)
	}
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	require.NoError(t, err)
	if _, err = pc.AddTrack(audioTrack); err != nil {
		t.Fatal(err)
	}
	offer, err := pc.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, pc.SetLocalDescription(offer))
	return pc, videoTrack, audioTrack, offer
}

// TestWHIPWorkerAnswersOffer verifies the WHIP worker's answer back-channel: from
// an offer it builds the PeerConnection, generates the SDP answer, and emits it
// as the FIRST frame on its socket — the synchronous reply main returns to the
// WHIP client. (Media flow → signed segments rides the same webRTCIngestPipeline
// the in-process path uses, plus the transcoder/frame machinery the fMP4 ingest tests
// already cover.)
func TestWHIPWorkerAnswersOffer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	clientPC, _, _, offer := whipClientOffer(t)
	defer clientPC.Close()

	sock := filepath.Join(t.TempDir(), "whip.sock")
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
		SocketPath:      sock,
		Transport:       IngestTransportWHIP,
		OfferSDP:        offer.SDP,
	}

	serveDone := make(chan error, 1)
	go func() { serveDone <- ServeWHIPIngestWorkerSocket(ctx, cfg) }()

	// Connect to the worker socket (retry until up) and read the Answer frame.
	dctx, dcancel := context.WithTimeout(ctx, 20*time.Second)
	defer dcancel()
	conn, derr := dialWorkerSocket(dctx, sock)
	require.NoError(t, derr)
	defer conn.Close()

	answerSDP, rerr := readWHIPAnswer(ingestframe.NewReader(conn))
	require.NoError(t, rerr, "worker emits an SDP answer as its first frame")
	require.Contains(t, answerSDP, "v=0", "valid SDP answer")

	// The answer must apply cleanly as the client's remote description — i.e. it's
	// a real, negotiated answer to the offer.
	require.NoError(t, clientPC.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer, SDP: answerSDP,
	}), "answer applies as the client's remote description")

	t.Logf("whip worker produced a %d-byte SDP answer", len(answerSDP))

	cancel()
	select {
	case <-serveDone:
	case <-time.After(25 * time.Second):
		t.Fatal("worker did not exit after cancel")
	}
}

// produceWHIPMedia streams synthetic H264 + Opus into the WHIP client's tracks
// via a gst encode pipeline until ctx is cancelled — i.e. a real WHIP publisher.
func produceWHIPMedia(t *testing.T, ctx context.Context, video, audio *webrtc.TrackLocalStaticSample) {
	t.Helper()
	desc := strings.Join([]string{
		"videotestsrc is-live=true ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=15 tune=zerolatency speed-preset=ultrafast ! h264parse ! video/x-h264,stream-format=byte-stream,alignment=au ! appsink name=vsink",
		"audiotestsrc is-live=true ! audioconvert ! audioresample ! opusenc ! opusparse ! appsink name=asink",
	}, "\n")
	pipeline, err := gst.NewPipelineFromString(desc)
	require.NoError(t, err)

	pump := func(name string, track *webrtc.TrackLocalStaticSample, dur time.Duration) {
		ele, gerr := pipeline.GetElementByName(name)
		require.NoError(t, gerr)
		app.SinkFromElement(ele).SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
				sample := sink.PullSample()
				if sample == nil {
					return gst.FlowEOS
				}
				buf := sample.GetBuffer()
				data := buf.Map(gst.MapRead).Bytes()
				buf.Unmap()
				if werr := track.WriteSample(pionmedia.Sample{Data: data, Duration: dur}); werr != nil {
					return gst.FlowError
				}
				return gst.FlowOK
			},
		})
	}
	pump("vsink", video, time.Second/30)
	pump("asink", audio, 20*time.Millisecond)

	go func() {
		<-ctx.Done()
		_ = pipeline.SetState(gst.StateNull)
	}()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
}

// TestWHIPWorkerLoopback is the full WHIP media path: a pion client offers,
// connects to the worker (which owns the PeerConnection), and streams real
// H264+Opus RTP; the worker must mux+sign+transcode it and serve a valid signed
// dual-codec segment over its socket — the WHIP parity of the fMP4 ingest worker e2e
// test.
func TestWHIPWorkerLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)

	clientPC, videoTrack, audioTrack, offer := whipClientOffer(t)
	defer clientPC.Close()

	connected := make(chan struct{})
	var once sync.Once
	clientPC.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateConnected {
			once.Do(func() { close(connected) })
		}
	})

	sock := filepath.Join(t.TempDir(), "whip.sock")
	cfg := IngestWorkerConfig{
		StreamerDID:     ms.Streamer(),
		KeyPEM:          keyPEM,
		CertPEM:         ms.Cert,
		Manifest:        manifest,
		NodeCertPEM:     ms.Cert,
		NodeKeyPEM:      keyPEM,
		BroadcasterHost: "test.example.com",
		SocketPath:      sock,
		Transport:       IngestTransportWHIP,
		OfferSDP:        offer.SDP,
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- ServeWHIPIngestWorkerSocket(ctx, cfg) }()

	dctx, dcancel := context.WithTimeout(ctx, 20*time.Second)
	defer dcancel()
	conn, derr := dialWorkerSocket(dctx, sock)
	require.NoError(t, derr)
	defer conn.Close()

	// One Reader for the whole connection: the streaming decoder reads ahead, so
	// the Answer and the segments after it must come through the same Reader (the
	// production WHIP path does the same).
	fr := ingestframe.NewReader(conn)
	answerSDP, rerr := readWHIPAnswer(fr)
	require.NoError(t, rerr)
	require.NoError(t, clientPC.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answerSDP}))

	select {
	case <-connected:
	case <-time.After(20 * time.Second):
		t.Fatal("client PC did not connect to the worker")
	}

	produceWHIPMedia(t, ctx, videoTrack, audioTrack)

	// Read signed segments; require at least one valid dual-codec one.
	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	var segs int
	for segs == 0 {
		typ, payload, ferr := fr.ReadFrame()
		require.NoError(t, ferr, "reading worker frames")
		if typ != ingestframe.Segment {
			continue
		}
		out, verr := muxl.RunMuxlVerify(ctx, bytes.NewReader(payload))
		require.NoError(t, verr)
		require.NotContains(t, out, `"validation_state":"Invalid"`, "segment must validate")
		segs++
	}
	_ = conn.SetReadDeadline(time.Time{})
	t.Logf("whip worker produced %d signed segment(s) from real RTP media", segs)

	// Close the connection before tearing down: in production main's connection
	// breaks on shutdown, which detaches the worker's frame server so it drains
	// into its buffer instead of blocking on an unread socket.
	conn.Close()
	cancel()
	select {
	case <-serveDone:
	case <-time.After(25 * time.Second):
		t.Fatal("worker did not exit after cancel")
	}
}
