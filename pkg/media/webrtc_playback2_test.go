package media

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
)

func TestObserveWebRTCSegmentLatency(t *testing.T) {
	streamer := "phase1-segment-latency-metrics"
	render := WebRTCSourceRendition
	sourceStart := time.Unix(100, 0)
	timing := &bus.SegmentTiming{
		SourceStart:    sourceStart,
		IngestReceived: sourceStart.Add(200 * time.Millisecond),
		Ingress:        "whip",
	}
	firstLabels := map[string]string{
		"streamer": streamer, "ingress": "whip", "rendition": render, "stage": "first_rtp",
	}
	completeLabels := map[string]string{
		"streamer": streamer, "ingress": "whip", "rendition": render, "stage": "complete",
	}
	beforeSourceFirst, beforeSourceFirstSum := histogramSample(t, "streamplace_webrtc_segment_source_age_ms", firstLabels)
	beforeSourceComplete, beforeSourceCompleteSum := histogramSample(t, "streamplace_webrtc_segment_source_age_ms", completeLabels)
	beforeIngestFirst, beforeIngestFirstSum := histogramSample(t, "streamplace_webrtc_segment_ingest_latency_ms", firstLabels)
	beforeIngestComplete, beforeIngestCompleteSum := histogramSample(t, "streamplace_webrtc_segment_ingest_latency_ms", completeLabels)

	observeWebRTCSegmentLatency(timing, streamer, render, sourceStart.Add(900*time.Millisecond), "first_rtp")
	observeWebRTCSegmentLatency(timing, streamer, render, sourceStart.Add(1800*time.Millisecond), "complete")

	sourceFirst, sourceFirstSum := histogramSample(t, "streamplace_webrtc_segment_source_age_ms", firstLabels)
	sourceComplete, sourceCompleteSum := histogramSample(t, "streamplace_webrtc_segment_source_age_ms", completeLabels)
	ingestFirst, ingestFirstSum := histogramSample(t, "streamplace_webrtc_segment_ingest_latency_ms", firstLabels)
	ingestComplete, ingestCompleteSum := histogramSample(t, "streamplace_webrtc_segment_ingest_latency_ms", completeLabels)
	require.Equal(t, beforeSourceFirst+1, sourceFirst)
	require.Equal(t, beforeSourceComplete+1, sourceComplete)
	require.Equal(t, beforeIngestFirst+1, ingestFirst)
	require.Equal(t, beforeIngestComplete+1, ingestComplete)
	require.InDelta(t, beforeSourceFirstSum+900, sourceFirstSum, 0.01)
	require.InDelta(t, beforeSourceCompleteSum+1800, sourceCompleteSum, 0.01)
	require.InDelta(t, beforeIngestFirstSum+700, ingestFirstSum, 0.01)
	require.InDelta(t, beforeIngestCompleteSum+1600, ingestCompleteSum, 0.01)
}

func TestPlaybackRateUsesSourceAge(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want float64
	}{
		{name: "target", age: 2 * time.Second, want: 1.0},
		{name: "target ceiling", age: 4 * time.Second, want: 1.0},
		{name: "soft catch-up", age: 5 * time.Second, want: 1.125},
		{name: "aggressive catch-up boundary", age: 6 * time.Second, want: 1.25},
		{name: "drop boundary", age: 8 * time.Second, want: 1.5},
		{name: "capped", age: 30 * time.Second, want: 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.InDelta(t, tt.want, getPlaybackRateForSourceAge(tt.age), 0.0001)
		})
	}
}

func TestPlaybackRecoveryUsesSourceAgeWhenLocalQueueIsShort(t *testing.T) {
	require.Greater(t, playbackRate(5*time.Second, 500*time.Millisecond, true), 1.0)
	require.True(t, shouldDropStaleGOP(8*time.Second+time.Nanosecond, true))
	require.False(t, shouldDropStaleGOP(8*time.Second+time.Nanosecond, false))
}

func TestPlaybackRecoveryFallsBackToLocalQueueWithoutTiming(t *testing.T) {
	require.Equal(t, getPlaybackRate(8*time.Second), playbackRate(0, 8*time.Second, false))
	require.False(t, shouldDropStaleGOP(30*time.Second, false))
	_, valid := playbackSourceAge(&bus.SegmentTiming{SourceStart: time.Unix(101, 0)}, time.Unix(100, 0))
	require.False(t, valid, "future source timestamps must not trigger recovery")
}

func TestDiscardStaleGOPsDropsOnlyWholeSegments(t *testing.T) {
	now := time.Unix(100, 0)
	stale := func(id string, age time.Duration) *bus.PacketizedSegment {
		return &bus.PacketizedSegment{
			Streamer:  "recovery-test",
			Rendition: WebRTCSourceRendition,
			Timing: &bus.SegmentTiming{
				SegmentID:   id,
				SourceStart: now.Add(-age),
			},
		}
	}
	packets := []*bus.PacketizedSegment{
		stale("stale-1", 9*time.Second),
		stale("stale-2", 8*time.Second+time.Millisecond),
		stale("live-edge", 2*time.Second),
	}
	packets[0].Duration = time.Second
	packets[1].Duration = 2 * time.Second
	packets[2].Duration = 3 * time.Second
	next := 0
	got, dropped, queuedDuration := discardStaleGOPs(packets[0], func() (*bus.PacketizedSegment, bool) {
		if next >= len(packets)-1 {
			return nil, false
		}
		next++
		return packets[next], true
	}, now)
	require.Equal(t, 2, dropped)
	require.Same(t, packets[2], got)
	require.Equal(t, packets[1].Duration, queuedDuration,
		"only stale packets removed from the queue after the consumer dequeues the first packet")
}

func TestDiscardStaleGOPsKeepsLastSegmentWithoutReplacement(t *testing.T) {
	now := time.Unix(100, 0)
	packet := &bus.PacketizedSegment{
		Streamer:  "recovery-test",
		Rendition: WebRTCSourceRendition,
		Timing: &bus.SegmentTiming{
			SegmentID:   "stale-last",
			SourceStart: now.Add(-9 * time.Second),
		},
	}

	got, dropped, queuedDuration := discardStaleGOPs(packet, func() (*bus.PacketizedSegment, bool) {
		return nil, false
	}, now)
	require.Same(t, packet, got)
	require.Zero(t, dropped)
	require.Zero(t, queuedDuration)
}

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

func TestPublishImmediateWebRTCSource(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	segs := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segs)

	const streamer = "phase2-immediate-webrtc"
	b := bus.NewBus()
	mm := &MediaManager{bus: b, cli: &config.CLI{}}
	timing := &bus.SegmentTiming{
		SegmentID:   "phase2-immediate-segment",
		SourceStart: time.Now().Add(-500 * time.Millisecond),
	}
	sub := b.SubscribeSegment(ctx, streamer, WebRTCSourceRendition)
	defer b.UnsubscribeSegment(ctx, streamer, WebRTCSourceRendition, sub)

	require.NoError(t, mm.publishImmediateWebRTC(ctx, streamer, true, timing, segs[0]))
	select {
	case got := <-sub.C:
		require.Equal(t, WebRTCSourceRendition, got.Rendition)
		require.NotNil(t, got.PacketizedData)
		require.NotEmpty(t, got.PacketizedData.Video)
		require.NotEmpty(t, got.PacketizedData.Audio)
		require.Equal(t, timing, got.Timing)
	case <-time.After(5 * time.Second):
		t.Fatal("immediate WebRTC source was not published")
	}

	canonicalSub := b.SubscribeSegment(ctx, streamer, "source")
	defer b.UnsubscribeSegment(ctx, streamer, "source", canonicalSub)
	select {
	case <-canonicalSub.C:
		t.Fatal("immediate WebRTC source must not enter the canonical source bus")
	default:
	}
}

func TestWebRTCPlaybackRecordsLiveCaptureTiming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mm, _ := getStaticTestMediaManager(t)
	mm.cli.WideOpen = true
	// Keep this loopback timing test independent of external STUN latency.
	mm.webrtcConfig.ICEServers = nil
	const streamer = "phase1-live-capture"
	const rendition = WebRTCSourceRendition

	inputFile, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	defer inputFile.Close()
	data, err := io.ReadAll(inputFile)
	require.NoError(t, err)

	now := time.Now()
	packetize := func(segmentID string, sourceStart, distributed time.Time) *bus.PacketizedSegment {
		timing := &bus.SegmentTiming{
			SegmentID:      segmentID,
			SourceStart:    sourceStart,
			IngestReceived: sourceStart.Add(50 * time.Millisecond),
			Distributed:    distributed,
			Ingress:        "whip",
		}
		packet, err := Packetize(ctx, mm.cli, &bus.Seg{
			Data:      data,
			Streamer:  streamer,
			Rendition: rendition,
			Timing:    timing,
		})
		require.NoError(t, err)
		require.NotEmpty(t, packet.Video)
		require.NotEmpty(t, packet.Audio)
		return packet
	}
	oldPacket := packetize("phase1-live-capture-old", now.Add(-1700*time.Millisecond), now.Add(-1650*time.Millisecond))
	packet := packetize("phase1-live-capture-latest", now.Add(-700*time.Millisecond), now.Add(-650*time.Millisecond))

	beforeFirstSent, beforeFirstSentSum := histogramSample(t, "streamplace_webrtc_source_age_ms", map[string]string{
		"streamer":  streamer,
		"rendition": rendition,
		"stage":     "first_sent",
	})
	beforeQueue := histogramSampleCount(t, "streamplace_webrtc_queue_duration_ms", map[string]string{
		"streamer":  streamer,
		"rendition": rendition,
	})
	beforeSetupToFirstSend := histogramSampleCount(t, "streamplace_webrtc_setup_to_first_send_ms", map[string]string{
		"streamer":  streamer,
		"rendition": rendition,
	})
	beforeVideoSend := histogramSampleCount(t, "streamplace_webrtc_video_send_duration_ms", map[string]string{
		"streamer":  streamer,
		"rendition": rendition,
	})
	beforeAudioSend := histogramSampleCount(t, "streamplace_webrtc_audio_send_duration_ms", map[string]string{
		"streamer":  streamer,
		"rendition": rendition,
	})
	beforeSegmentFirstRTP := histogramSampleCount(t, "streamplace_webrtc_segment_source_age_ms", map[string]string{
		"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "first_rtp",
	})
	beforeSegmentComplete := histogramSampleCount(t, "streamplace_webrtc_segment_source_age_ms", map[string]string{
		"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "complete",
	})

	mm.bus.PublishSegment(ctx, streamer, WebRTCSourceRendition, &bus.Seg{
		Data:           data,
		PacketizedData: oldPacket,
		Published:      true,
		Streamer:       streamer,
		Rendition:      WebRTCSourceRendition,
		Timing:         oldPacket.Timing,
	})
	mm.bus.PublishSegment(ctx, streamer, WebRTCSourceRendition, &bus.Seg{
		Data:           data,
		PacketizedData: packet,
		Published:      true,
		Streamer:       streamer,
		Rendition:      WebRTCSourceRendition,
		Timing:         packet.Timing,
	})

	receiverAPI, receiverConfig, err := newWebRTCAPI()
	require.NoError(t, err)
	receiverConfig.ICEServers = nil
	receiver, err := receiverAPI.NewPeerConnection(receiverConfig)
	require.NoError(t, err)
	defer receiver.Close()

	connected := make(chan struct{})
	var connectedOnce sync.Once
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			connectedOnce.Do(func() { close(connected) })
		}
	})
	received := make(chan struct{})
	var receivedOnce sync.Once
	receiver.OnTrack(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
		receivedOnce.Do(func() { close(received) })
	})

	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
	require.NoError(t, err)
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	require.NoError(t, err)
	offer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(offer))
	gatherComplete := webrtc.GatheringCompletePromise(receiver)
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		t.Fatal("receiver ICE gathering timed out")
	}

	answer, err := mm.WebRTCPlayback2(ctx, streamer, rendition, receiver.LocalDescription(), "")
	require.NoError(t, err)
	require.NoError(t, receiver.SetRemoteDescription(*answer))

	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("local WebRTC playback did not connect")
	}
	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("local WebRTC receiver did not receive RTP")
	}

	require.Eventually(t, func() bool {
		return histogramSampleCount(t, "streamplace_webrtc_source_age_ms", map[string]string{
			"streamer": streamer, "rendition": rendition, "stage": "first_sent",
		}) > beforeFirstSent &&
			histogramSampleCount(t, "streamplace_webrtc_queue_duration_ms", map[string]string{
				"streamer": streamer, "rendition": rendition,
			}) > beforeQueue &&
			histogramSampleCount(t, "streamplace_webrtc_setup_to_first_send_ms", map[string]string{
				"streamer": streamer, "rendition": rendition,
			}) > beforeSetupToFirstSend &&
			histogramSampleCount(t, "streamplace_webrtc_video_send_duration_ms", map[string]string{
				"streamer": streamer, "rendition": rendition,
			}) > beforeVideoSend &&
			histogramSampleCount(t, "streamplace_webrtc_audio_send_duration_ms", map[string]string{
				"streamer": streamer, "rendition": rendition,
			}) > beforeAudioSend &&
			histogramSampleCount(t, "streamplace_webrtc_segment_source_age_ms", map[string]string{
				"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "first_rtp",
			}) > beforeSegmentFirstRTP &&
			histogramSampleCount(t, "streamplace_webrtc_segment_source_age_ms", map[string]string{
				"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "complete",
			}) > beforeSegmentComplete
	}, 5*time.Second, 25*time.Millisecond, "playback should emit first-send and sender timing metrics")

	firstSentCount, firstSentSum := histogramSample(t, "streamplace_webrtc_source_age_ms", map[string]string{
		"streamer": streamer, "rendition": rendition, "stage": "first_sent",
	})
	require.Equal(t, uint64(1), firstSentCount-beforeFirstSent, "startup should replay only the latest cached segment")
	require.Less(t, (firstSentSum-beforeFirstSentSum)/float64(firstSentCount-beforeFirstSent), 1000.0,
		"first RTP source age should stay below one second")

	for _, metric := range []struct {
		name   string
		labels map[string]string
	}{
		{"streamplace_packetize_duration_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_packetize_queue_duration_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_packetize_source_age_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_webrtc_source_age_ms", map[string]string{"streamer": streamer, "rendition": rendition, "stage": "queued"}},
		{"streamplace_webrtc_source_age_ms", map[string]string{"streamer": streamer, "rendition": rendition, "stage": "first_sent"}},
		{"streamplace_webrtc_queue_duration_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_webrtc_setup_to_first_send_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_webrtc_video_send_duration_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_webrtc_audio_send_duration_ms", map[string]string{"streamer": streamer, "rendition": rendition}},
		{"streamplace_webrtc_segment_source_age_ms", map[string]string{"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "first_rtp"}},
		{"streamplace_webrtc_segment_source_age_ms", map[string]string{"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "complete"}},
		{"streamplace_webrtc_segment_ingest_latency_ms", map[string]string{"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "first_rtp"}},
		{"streamplace_webrtc_segment_ingest_latency_ms", map[string]string{"streamer": streamer, "ingress": "whip", "rendition": rendition, "stage": "complete"}},
	} {
		count, sum := histogramSample(t, metric.name, metric.labels)
		require.Positive(t, count, "expected baseline sample for %s", metric.name)
		stage := metric.labels["stage"]
		if stage != "" {
			stage = " stage=" + stage
		}
		t.Logf("baseline metric=%s%s count=%d sum_ms=%.1f avg_ms=%.1f", metric.name, stage, count, sum, sum/float64(count))
	}

	// A later dual-codec replacement for the same logical segment must not make
	// WebRTC play that GOP twice after the immediate Opus path has already sent it.
	mm.bus.PublishSegment(ctx, streamer, WebRTCSourceRendition, &bus.Seg{
		Data:           data,
		PacketizedData: packet,
		Published:      true,
		Streamer:       streamer,
		Rendition:      WebRTCSourceRendition,
		Timing:         packet.Timing,
	})
	require.Never(t, func() bool {
		count, _ := histogramSample(t, "streamplace_webrtc_source_age_ms", map[string]string{
			"streamer": streamer, "rendition": WebRTCSourceRendition, "stage": "first_sent",
		})
		return count > beforeFirstSent+1
	}, 500*time.Millisecond, 25*time.Millisecond, "dual-codec replacement must be deduplicated")
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
