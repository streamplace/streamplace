package media

import (
	"context"
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestWebRTCPlayback2(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)
	offer := &webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  firefoxNoH264SDP,
	}
	answer, err := mm.WebRTCPlayback2(context.Background(), "test-user", "test-rendition", offer)
	require.ErrorContains(t, err, "RTPSender created with no codecs")
	require.Nil(t, answer)
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
