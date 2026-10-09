package media

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pion/webrtc/v4"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
)

func TestWebRTCPlayback2(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	offer := &webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  firefoxNoH264SDP,
	}
	answer, err := mm.WebRTCPlayback2(context.Background(), "test-user", "test-rendition", offer, "")
	require.ErrorContains(t, err, "RTPSender created with no codecs")
	require.Nil(t, answer)
}

// Cancelling playback must release its segment subscription even while the
// sender is pacing a real segment and its pending segment queue is full.
func TestWebRTCPlayback2CancelsBackloggedPlayback(t *testing.T) {
	var logs logCapture
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError})))
	defer slog.SetDefault(previousLogger)

	previousVerbosity := flag.Lookup("v").Value.String()
	require.NoError(t, flag.Set("v", "3"))
	t.Cleanup(func() { require.NoError(t, flag.Set("v", previousVerbosity)) })
	var packet *bus.PacketizedSegment
	withNoGSTLeaks(t, func() { packet = playbackPacketFixture(t) })

	mm := loopbackPlaybackManager()
	ignore := goleak.IgnoreCurrent()
	defer func() {
		goleak.VerifyNone(t, ignore)
		require.NotContains(t, logs.String(), "level=ERROR")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := startBackloggedPlayback(t, ctx, mm, t.Name(), packet)
	defer client.Close()

	cancel()
	require.NoError(t, client.Close())
	subscriptions := spmetrics.SegmentSubscriptionsOpen.WithLabelValues(t.Name(), WebRTCSourceRendition)
	var metric dto.Metric
	require.Eventually(t, func() bool { return subscriptions.Write(&metric) == nil && metric.GetGauge().GetValue() == 0 }, time.Second, time.Millisecond,
		"cancelled playback releases its segment subscription")
}

func playbackPacketFixture(tb testing.TB) *bus.PacketizedSegment {
	tb.Helper()
	gstinit.InitGST()
	fixture, err := os.ReadFile(getFixture("sample-segment.mp4"))
	require.NoError(tb, err)
	packet, err := Packetize(context.Background(), &config.CLI{}, &bus.Seg{Data: fixture})
	require.NoError(tb, err)
	require.NotEmpty(tb, packet.Audio)
	require.Positive(tb, packet.Duration)
	return packet
}

func loopbackPlaybackManager() *MediaManager {
	settings := webrtc.SettingEngine{}
	settings.SetIncludeLoopbackCandidate(true)
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	settings.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	return &MediaManager{
		cli:       &config.CLI{},
		bus:       bus.NewBus(),
		webrtcAPI: webrtc.NewAPI(webrtc.WithSettingEngine(settings)),
	}
}

func startBackloggedPlayback(tb testing.TB, ctx context.Context, mm *MediaManager, user string, packet *bus.PacketizedSegment) *webrtc.PeerConnection {
	tb.Helper()
	client, err := mm.webrtcAPI.NewPeerConnection(webrtc.Configuration{})
	require.NoError(tb, err)
	started := false
	defer func() {
		if !started {
			client.Close()
		}
	}()
	_, err = client.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	require.NoError(tb, err)
	offer, err := client.CreateOffer(nil)
	require.NoError(tb, err)
	require.NoError(tb, client.SetLocalDescription(offer))
	select {
	case <-webrtc.GatheringCompletePromise(client):
	case <-ctx.Done():
		tb.Fatal("client ICE gathering timed out")
	}

	seg := &bus.Seg{Published: true, PacketizedData: packet}
	for range 2 {
		mm.bus.PublishSegment(ctx, user, WebRTCSourceRendition, seg)
	}
	answer, err := mm.WebRTCPlayback2(ctx, user, renditions.AudioRendition.Name, client.LocalDescription(), "")
	require.NoError(tb, err)
	require.NotNil(tb, answer)
	require.NoError(tb, client.SetRemoteDescription(*answer))

	// Wait for the connected session before filling its live segment queue.
	subscriptions := spmetrics.SegmentSubscriptionsOpen.WithLabelValues(user, WebRTCSourceRendition)
	var metric dto.Metric
	require.Eventually(tb, func() bool { return subscriptions.Write(&metric) == nil && metric.GetGauge().GetValue() == 1 }, time.Second, time.Millisecond)
	for range 16 {
		for range 256 {
			mm.bus.PublishSegment(ctx, user, WebRTCSourceRendition, seg)
		}
		// Let the reader fill its queue before the next burst fills the bus.
		time.Sleep(5 * time.Millisecond)
	}
	started = true
	return client
}

func BenchmarkWebRTCPlaybackBacklog(b *testing.B) {
	packet := playbackPacketFixture(b)
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(previousLogger)
	mm := loopbackPlaybackManager()
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(b, ignore)
	parent, cancelParent := context.WithCancel(b.Context())
	defer cancelParent()
	subscriptions := spmetrics.SegmentSubscriptionsOpen.WithLabelValues(b.Name(), WebRTCSourceRendition)
	var metric dto.Metric
	var cancelTime time.Duration
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithCancel(parent)
		client := startBackloggedPlayback(b, ctx, mm, b.Name(), packet)
		start := time.Now()
		cancel()
		require.NoError(b, client.Close())
		for {
			require.NoError(b, subscriptions.Write(&metric))
			if metric.GetGauge().GetValue() == 0 {
				break
			}
			if time.Since(start) >= time.Second {
				b.Fatal("cancelled playback did not release its segment subscription")
			}
			runtime.Gosched()
		}
		cancelTime += time.Since(start)
	}
	b.ReportMetric(float64(cancelTime)/float64(b.N), "cancel-ns/op")
}

func TestWriteSamplesPacing(t *testing.T) {
	var packet *bus.PacketizedSegment
	withNoGSTLeaks(t, func() { packet = playbackPacketFixture(t) })
	require.NotEmpty(t, packet.Video)
	for _, testCase := range []struct {
		name      string
		durations []time.Duration
		scalar    float64
		elapsed   time.Duration
	}{
		{name: "empty", scalar: 1},
		{name: "zero durations", durations: []time.Duration{0, 0, 0}, scalar: 1},
		{name: "nonuniform", durations: []time.Duration{0, 30 * time.Millisecond, 60 * time.Millisecond, 0, 15 * time.Millisecond}, scalar: 1, elapsed: 105 * time.Millisecond},
		{name: "accelerated", durations: []time.Duration{0, 30 * time.Millisecond, 60 * time.Millisecond, 0, 15 * time.Millisecond}, scalar: 1.5, elapsed: 70 * time.Millisecond},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "stream")
				require.NoError(t, err)
				samples := make([]bus.PacketizedSample, len(testCase.durations))
				for i, duration := range testCase.durations {
					samples[i] = bus.PacketizedSample{Data: packet.Video[0].Data, Duration: duration}
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				start := time.Now()
				go func() { done <- writeSamples(ctx, track, samples, testCase.scalar) }()
				synctest.Wait()
				if testCase.elapsed > 0 {
					time.Sleep(testCase.elapsed - time.Nanosecond)
					synctest.Wait()
					select {
					case <-done:
						t.Fatal("pacing completed before the scheduled deadline")
					default:
					}
					time.Sleep(time.Nanosecond)
					synctest.Wait()
				}
				select {
				case err := <-done:
					require.NoError(t, err)
				default:
					t.Fatal("pacing did not complete at the scheduled deadline")
				}
				require.Equal(t, testCase.elapsed, time.Since(start))
			})
		})
	}
}

func TestWriteAudioSamplesUsesSegmentDuration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000}, "audio", "stream")
		require.NoError(t, err)
		packet := &bus.PacketizedSegment{
			Duration: 40 * time.Millisecond,
			Audio: []bus.PacketizedSample{
				{Duration: 100 * time.Millisecond},
				{Duration: 20 * time.Millisecond},
			},
		}
		start := time.Now()
		require.NoError(t, writeAudioSamples(t.Context(), track, packet, 1))
		require.Equal(t, packet.Duration, time.Since(start), "timestamp gaps must not expand the audio packet clock")
		packet.Duration = 0
		start = time.Now()
		require.NoError(t, writeAudioSamples(t.Context(), track, packet, 1))
		require.Zero(t, time.Since(start), "a missing segment duration must not start audio pacing")
	})
}

func TestWriteSamplesCancelsDuringWait(t *testing.T) {
	var packet *bus.PacketizedSegment
	withNoGSTLeaks(t, func() { packet = playbackPacketFixture(t) })
	require.NotEmpty(t, packet.Video)
	synctest.Test(t, func(t *testing.T) {
		track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "stream")
		require.NoError(t, err)
		samples := []bus.PacketizedSample{
			{Data: packet.Video[0].Data, Duration: 30 * time.Millisecond},
			{Data: packet.Video[0].Data, Duration: 60 * time.Millisecond},
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- writeSamples(ctx, track, samples, 1) }()
		synctest.Wait()
		time.Sleep(45 * time.Millisecond)
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("pacing completed before cancellation")
		default:
		}
		cancel()
		synctest.Wait()
		select {
		case err := <-done:
			require.NoError(t, err)
		default:
			t.Fatal("cancellation did not stop the pending wait")
		}
		require.Equal(t, 45*time.Millisecond, time.Since(start))
	})
}

func BenchmarkWriteSamplesVideo(b *testing.B) {
	packet := playbackPacketFixture(b)
	require.NotEmpty(b, packet.Video)
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "stream")
	require.NoError(b, err)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if err := writeSamples(ctx, track, packet.Video, 1); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(packet.Video)), "samples/op")
}

var firefoxNoH264SDP = `v=0
o=mozilla...THIS_IS_SDPARTA-99.0 2400864153024665403 0 IN IP4 0.0.0.0
s=-
t=0 0
a=sendrecv
a=fingerprint:sha-256 9A:55:EE:77:40:E7:C9:7F:DB:1A:D4:33:7C:06:9B:07:AE:CE:0F:06:52:1E:DE:5B:8B:A6:65:4C:48:C6:73:15
a=group:BUNDLE 0 1
a=ice-options:trickle
a=msid-semantic:WMS *
m=video 9 UDP/TLS/RTP/SAVPF 120 124 121 125 99 100 123 122 119
c=IN IP4 0.0.0.0
a=candidate:0 1 UDP 2122252543 f2fe908b-a599-4c2b-8e5b-411fb045e57a.local 58949 typ host
a=candidate:1 1 TCP 2105524479 f2fe908b-a599-4c2b-8e5b-411fb045e57a.local 9 typ host tcptype active
a=candidate:0 2 UDP 2122252542 f2fe908b-a599-4c2b-8e5b-411fb045e57a.local 52046 typ host
a=candidate:1 2 TCP 2105524478 f2fe908b-a599-4c2b-8e5b-411fb045e57a.local 9 typ host tcptype active
a=recvonly
a=end-of-candidates
a=extmap:3 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:5 urn:ietf:params:rtp-hdrext:toffset
a=extmap:6/recvonly http://www.webrtc.org/experiments/rtp-hdrext/playout-delay
a=extmap:7 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=extmap-allow-mixed
a=fmtp:120 max-fs=12288;max-fr=60
a=fmtp:124 apt=120
a=fmtp:121 max-fs=12288;max-fr=60
a=fmtp:125 apt=121
a=fmtp:100 apt=99
a=fmtp:119 apt=122
a=ice-pwd:073b56d923776f7889e9f31dd122eaf5
a=ice-ufrag:9ba3b1bf
a=mid:0
a=rtcp:52046 IN IP4 f2fe908b-a599-4c2b-8e5b-411fb045e57a.local
a=rtcp-fb:120 nack
a=rtcp-fb:120 nack pli
a=rtcp-fb:120 ccm fir
a=rtcp-fb:120 goog-remb
a=rtcp-fb:120 transport-cc
a=rtcp-fb:121 nack
a=rtcp-fb:121 nack pli
a=rtcp-fb:121 ccm fir
a=rtcp-fb:121 goog-remb
a=rtcp-fb:121 transport-cc
a=rtcp-fb:99 nack
a=rtcp-fb:99 nack pli
a=rtcp-fb:99 ccm fir
a=rtcp-fb:99 goog-remb
a=rtcp-fb:99 transport-cc
a=rtcp-fb:123 nack
a=rtcp-fb:123 nack pli
a=rtcp-fb:123 ccm fir
a=rtcp-fb:123 goog-remb
a=rtcp-fb:123 transport-cc
a=rtcp-fb:122 nack
a=rtcp-fb:122 nack pli
a=rtcp-fb:122 ccm fir
a=rtcp-fb:122 goog-remb
a=rtcp-fb:122 transport-cc
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:120 VP8/90000
a=rtpmap:124 rtx/90000
a=rtpmap:121 VP9/90000
a=rtpmap:125 rtx/90000
a=rtpmap:99 AV1/90000
a=rtpmap:100 rtx/90000
a=rtpmap:123 ulpfec/90000
a=rtpmap:122 red/90000
a=rtpmap:119 rtx/90000
a=setup:actpass
a=ssrc:3998233880 cname:{d6f4dc1f-0f71-4a23-8ba7-8c95370e5479}
m=audio 0 UDP/TLS/RTP/SAVPF 109 9 0 8 101
c=IN IP4 0.0.0.0
a=bundle-only
a=recvonly
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=extmap:2/recvonly urn:ietf:params:rtp-hdrext:csrc-audio-level
a=extmap:3 urn:ietf:params:rtp-hdrext:sdes:mid
a=extmap-allow-mixed
a=fmtp:109 maxplaybackrate=48000;stereo=1;useinbandfec=1
a=fmtp:101 0-15
a=ice-pwd:073b56d923776f7889e9f31dd122eaf5
a=ice-ufrag:9ba3b1bf
a=mid:1
a=rtcp-mux
a=rtpmap:109 opus/48000/2
a=rtpmap:9 G722/8000/1
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:101 telephone-event/8000
a=setup:actpass
a=ssrc:543748180 cname:{d6f4dc1f-0f71-4a23-8ba7-8c95370e5479}
`
