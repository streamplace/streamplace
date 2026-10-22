package media

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abema/go-mp4"
	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/muxl"
)

// whipClientOffer builds a WHIP-style SDP offer (a videoMime video track +
// an Opus audio track), the way a real WHIP client does, returning the client
// PC, its tracks, and the offer. The client offers the video codecs offered
// names, in that order, or pion's defaults (H.264, AV1, VP8, VP9) when none.
func whipClientOffer(t *testing.T, videoMime string, offered ...string) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, *webrtc.TrackLocalStaticSample, webrtc.SessionDescription) {
	t.Helper()
	api := webrtc.NewAPI()
	if len(offered) > 0 {
		m := &webrtc.MediaEngine{}
		payloadTypes := map[string]webrtc.PayloadType{webrtc.MimeTypeH264: 102, webrtc.MimeTypeAV1: 45}
		for _, mime := range offered {
			require.NoError(t, m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mime, ClockRate: 90000},
				PayloadType:        payloadTypes[mime],
			}, webrtc.RTPCodecTypeVideo))
		}
		require.NoError(t, m.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
			PayloadType:        111,
		}, webrtc.RTPCodecTypeAudio))
		api = webrtc.NewAPI(webrtc.WithMediaEngine(m))
	}
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: videoMime}, "video", "pion")
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
// already cover.) The answer takes one video codec: H.264 whenever the
// publisher offers it, even listing AV1 first, else AV1.
func TestWHIPWorkerAnswersOffer(t *testing.T) {
	for _, tc := range []struct {
		name      string
		videoMime string
		offered   []string
		answered  string
	}{
		{"pion defaults", webrtc.MimeTypeH264, nil, webrtc.MimeTypeH264},
		{"AV1 listed first", webrtc.MimeTypeH264, []string{webrtc.MimeTypeAV1, webrtc.MimeTypeH264}, webrtc.MimeTypeH264},
		{"AV1 only", webrtc.MimeTypeAV1, []string{webrtc.MimeTypeAV1}, webrtc.MimeTypeAV1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ms := newBareSegmentSigner(t)
			keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
			require.NoError(t, err)
			manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
			require.NoError(t, err)

			clientPC, _, _, offer := whipClientOffer(t, tc.videoMime, tc.offered...)
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
			answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answerSDP}
			require.Equal(t, map[string]bool{strings.ToLower(tc.answered): true}, offeredVideoMimeTypes(&answer))

			// The answer must apply cleanly as the client's remote description — i.e. it's
			// a real, negotiated answer to the offer.
			require.NoError(t, clientPC.SetRemoteDescription(answer), "answer applies as the client's remote description")

			t.Logf("whip worker produced a %d-byte SDP answer", len(answerSDP))

			cancel()
			select {
			case <-serveDone: //nolint:errcheck // only the exit matters
			case <-time.After(25 * time.Second):
				t.Fatal("worker did not exit after cancel")
			}
		})
	}
}

// produceWHIPMedia streams the video and audio of a gst pipeline desc (appsinks
// vsink and asink) into the WHIP client's tracks until ctx is cancelled — i.e.
// a real WHIP publisher.
func produceWHIPMedia(t *testing.T, ctx context.Context, desc string, video, audio *webrtc.TrackLocalStaticSample) {
	t.Helper()
	pipelineElem, err := gst.ParseLaunch(desc)
	require.NoError(t, err)
	pipeline := pipelineElem.(gst.Pipeline)

	pump := func(name string, track *webrtc.TrackLocalStaticSample) {
		ele := pipeline.GetByName(name)
		require.NotNil(t, ele)
		ele.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
			NewSample: func(sink app.AppSink) gst.FlowReturn {
				sample := sink.PullSample()
				if sample == nil {
					return gst.FlowEOS
				}
				data, cerr := copyBufferData(sample.GetBuffer())
				if cerr != nil {
					return gst.FlowError
				}
				dur, _ := clockTimeDuration(sample.GetBuffer().Duration())
				if werr := track.WriteSample(pionmedia.Sample{Data: data, Duration: dur}); werr != nil {
					return gst.FlowError
				}
				return gst.FlowOK
			},
		})
	}
	pump("vsink", video)
	pump("asink", audio)

	go func() {
		<-ctx.Done()
		_ = pipeline.SetState(gst.StateNull)
	}()
	require.NotEqual(t, gst.StateChangeFailure, pipeline.SetState(gst.StatePlaying))
}

// TestWHIPWorkerLoopback is the full WHIP media path: a pion client offers,
// connects to the worker (which owns the PeerConnection), and streams real
// RTP; the worker must mux+sign+transcode it and serve a valid signed
// dual-codec segment over its socket in the publisher's video codec — the
// WHIP parity of the fMP4 ingest worker e2e test. An AV1 publisher (offering
// only AV1) goes through the Go depacketizer.
func TestWHIPWorkerLoopback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		videoMime string
		offered   []string
		desc      string
		codec     VideoCodec
	}{
		{
			name:      "H.264",
			videoMime: webrtc.MimeTypeH264,
			desc: strings.Join([]string{
				"videotestsrc is-live=true ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=15 tune=zerolatency speed-preset=ultrafast ! h264parse ! video/x-h264,stream-format=byte-stream,alignment=au ! appsink name=vsink",
				"audiotestsrc is-live=true ! audioconvert ! audioresample ! opusenc ! opusparse ! appsink name=asink",
			}, "\n"),
			codec: VideoH264,
		},
		{
			name:      "AV1",
			videoMime: webrtc.MimeTypeAV1,
			offered:   []string{webrtc.MimeTypeAV1},
			// No AV1 encoder in the build: the fixture's AV1 track, paced
			// at realtime by the sinks' clock sync, as `streamplace whip`
			// sends it.
			desc: strings.Join([]string{
				fmt.Sprintf("filesrc location=%s ! qtdemux name=demux", getFixture("av1-h264-opus.mp4")),
				fmt.Sprintf("demux.video_0 ! queue ! %s ! %s ! appsink name=vsink", VideoAV1.Parser, VideoAV1.SampleCaps),
				"demux.audio_0 ! queue ! opusparse ! appsink name=asink",
			}, "\n"),
			codec: VideoAV1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ms := newBareSegmentSigner(t)
			keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
			require.NoError(t, err)
			manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
			require.NoError(t, err)

			clientPC, videoTrack, audioTrack, offer := whipClientOffer(t, tc.videoMime, tc.offered...)
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

			produceWHIPMedia(t, ctx, tc.desc, videoTrack, audioTrack)

			// Read signed segments; require at least one valid dual-codec one,
			// its video in the publisher's codec and, for AV1, timed by the RTP
			// timestamps (30 fps) and described by an av1C carrying the
			// publisher's sequence header: given no codec_data, qtmux writes one
			// of its own without it (and with two bytes it never initializes).
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
				packet, perr := Packetize(ctx, &config.CLI{}, &bus.Seg{Muxl: payload})
				require.NoError(t, perr)
				video := packet.VideoSamples(tc.codec.MimeType)
				require.NotEmpty(t, video)
				if tc.codec.Name == VideoAV1.Name {
					for i, s := range video[:len(video)-1] {
						require.InDelta(t, time.Second/30, s.Duration, float64(time.Millisecond), "frame %d", i)
					}
					var flat bytes.Buffer
					require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(payload), "flat", &flat))
					boxes, err := mp4.ExtractBoxWithPayload(bytes.NewReader(flat.Bytes()), nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeAv01(), mp4.BoxTypeAv1C()})
					require.NoError(t, err)
					require.Len(t, boxes, 1)
					require.NotNil(t, tuSequenceHeader(boxes[0].Payload.(*mp4.Av1C).ConfigOBUs))
				}
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
			case <-serveDone: //nolint:errcheck // only the exit matters
			case <-time.After(25 * time.Second):
				t.Fatal("worker did not exit after cancel")
			}
		})
	}
}
