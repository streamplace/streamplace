package media

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/bus"
)

// The shared WHIP receiver must keep requesting keyframes on a subsecond
// cadence after its initial PLI, so GOP delivery does not wait for the
// publisher's own keyframe interval.
func TestWebRTCReceiverRequestsKeyframesOnSubsecondCadence(t *testing.T) {
	var fixture *bus.PacketizedSegment
	withNoGSTLeaks(t, func() { fixture = playbackPacketFixture(t) })
	require.NotEmpty(t, fixture.Video)

	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	api, _, err := newWebRTCAPI()
	require.NoError(t, err)
	settings := webrtc.SettingEngine{}
	settings.LoggerFactory = logging.NewDefaultLoggerFactory()
	settings.SetIncludeLoopbackCandidate(true)
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	settings.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	webrtc.WithSettingEngine(settings)(api)
	receiver, err := api.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer receiver.Close()
	publisher, err := webrtc.NewAPI(webrtc.WithSettingEngine(settings)).NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer publisher.Close()
	video, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "whip")
	require.NoError(t, err)
	sender, err := publisher.AddTrack(video)
	require.NoError(t, err)

	connected := make(chan struct{}, 1)
	publisher.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case connected <- struct{}{}:
			default:
			}
		}
	})
	receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		for {
			if _, _, err := track.ReadRTP(); err != nil {
				return
			}
		}
	})
	offer, err := publisher.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, publisher.SetLocalDescription(offer))
	select {
	case <-webrtc.GatheringCompletePromise(publisher):
	case <-ctx.Done():
		t.Fatal("publisher ICE gathering timed out")
	}
	require.NoError(t, receiver.SetRemoteDescription(*publisher.LocalDescription()))
	answer, err := receiver.CreateAnswer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(answer))
	select {
	case <-webrtc.GatheringCompletePromise(receiver):
	case <-ctx.Done():
		t.Fatal("receiver ICE gathering timed out")
	}
	require.NoError(t, publisher.SetRemoteDescription(*receiver.LocalDescription()))
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("local WHIP peers did not connect")
	}

	encodings := sender.GetParameters().Encodings
	require.Len(t, encodings, 1)
	primarySSRC := uint32(encodings[0].SSRC)
	requests := make(chan *rtcp.PictureLossIndication, 8)
	readErrors := make(chan error, 1)
	go func() {
		for {
			packets, _, err := sender.ReadRTCP()
			if err != nil {
				readErrors <- err
				return
			}
			for _, packet := range packets {
				// The offer can also register an RTX repair stream with PLI feedback.
				if pli, ok := packet.(*rtcp.PictureLossIndication); ok && pli.MediaSSRC == primarySSRC {
					select {
					case requests <- pli:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	first := fixture.Video[0]
	require.NoError(t, video.WriteSample(pionmedia.Sample{Data: first.Data, Duration: first.Duration}))
	for request := range 4 {
		// Leave ample scheduler tolerance around the intended 400 ms cadence,
		// while rejecting the helper's default three-second interval.
		timer := time.NewTimer(900 * time.Millisecond)
		select {
		case <-requests:
			timer.Stop()
		case err := <-readErrors:
			timer.Stop()
			require.NoError(t, err, "read publisher RTCP")
		case <-timer.C:
			t.Fatalf("keyframe request %d did not arrive within a subsecond interval", request+1)
		case <-ctx.Done():
			timer.Stop()
			t.Fatal("keyframe cadence test timed out")
		}
	}
}
