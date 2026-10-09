package media

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
)

// A finite cached GOP must survive a delayed handshake, and its predecessor
// must not be replayed to a viewer joining the live edge.
func TestWebRTCPlaybackStartsWithLatestCachedSegment(t *testing.T) {
	var fixture *bus.PacketizedSegment
	withNoGSTLeaks(t, func() { fixture = playbackPacketFixture(t) })
	older := fixture.Audio[0].Data
	var latest []byte
	for _, sample := range fixture.Audio[1:] {
		if !bytes.Equal(sample.Data, older) {
			latest = sample.Data
			break
		}
	}
	require.NotEmpty(t, latest, "fixture needs distinguishable encoded Opus packets")
	for _, wait := range []time.Duration{0, 3 * time.Second} {
		t.Run(wait.String(), func(t *testing.T) {
			mm := loopbackPlaybackManager()
			ignore := goleak.IgnoreCurrent()
			defer goleak.VerifyNone(t, ignore)
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			for _, data := range [][]byte{older, latest} {
				packet := &bus.PacketizedSegment{Duration: time.Second}
				for range 50 {
					packet.Audio = append(packet.Audio, bus.PacketizedSample{Data: data, Duration: 20 * time.Millisecond})
				}
				mm.bus.PublishSegment(ctx, t.Name(), WebRTCSourceRendition, &bus.Seg{Published: true, PacketizedData: packet})
			}
			receiver, err := mm.webrtcAPI.NewPeerConnection(webrtc.Configuration{})
			require.NoError(t, err)
			defer receiver.Close()
			received := make(chan []byte, 1)
			receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
				packet, _, err := track.ReadRTP()
				if err == nil {
					received <- packet.Payload
				}
			})
			_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
			require.NoError(t, err)
			offer, err := receiver.CreateOffer(nil)
			require.NoError(t, err)
			require.NoError(t, receiver.SetLocalDescription(offer))
			select {
			case <-webrtc.GatheringCompletePromise(receiver):
			case <-ctx.Done():
				t.Fatal("receiver ICE gathering timed out")
			}
			answer, err := mm.WebRTCPlayback2(ctx, t.Name(), renditions.AudioRendition.Name, receiver.LocalDescription(), "")
			require.NoError(t, err)
			time.Sleep(wait)
			require.NoError(t, receiver.SetRemoteDescription(*answer))
			select {
			case payload := <-received:
				require.Equal(t, latest, payload, "first RTP must belong to the newest cached segment")
			case <-ctx.Done():
				t.Fatal("finite cached media was consumed before the receiver connected")
			}
		})
	}
}

// A caller abandoning its SDP answer must leave no subscription or peer alive.
func TestWebRTCPlaybackCancelsPendingHandshake(t *testing.T) {
	mm := loopbackPlaybackManager()
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	receiver, err := mm.webrtcAPI.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer receiver.Close()
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	require.NoError(t, err)
	offer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(offer))
	select {
	case <-webrtc.GatheringCompletePromise(receiver):
	case <-ctx.Done():
		t.Fatal("receiver ICE gathering timed out")
	}
	answer, err := mm.WebRTCPlayback2(ctx, t.Name(), renditions.AudioRendition.Name, receiver.LocalDescription(), "")
	require.NoError(t, err)
	require.NotNil(t, answer)
	subscriptions := spmetrics.SegmentSubscriptionsOpen.WithLabelValues(t.Name(), WebRTCSourceRendition)
	require.Never(t, func() bool {
		var metric dto.Metric
		return subscriptions.Write(&metric) != nil || metric.GetGauge().GetValue() != 0
	}, 100*time.Millisecond, time.Millisecond, "pending handshakes must not consume live segments")
	require.Zero(t, mm.bus.GetViewerCount(t.Name()))
	cancel()
	require.NoError(t, receiver.Close())
	var metric dto.Metric
	require.NoError(t, subscriptions.Write(&metric))
	require.Zero(t, metric.GetGauge().GetValue())
	require.Zero(t, mm.bus.GetViewerCount(t.Name()))
}
