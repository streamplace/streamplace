package rtcrec

import (
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type PeerConnection interface {
	AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error)
	Close() error
	SetRemoteDescription(description webrtc.SessionDescription) error
	CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error)
	SetLocalDescription(description webrtc.SessionDescription) error
	OnICEConnectionStateChange(func(webrtc.ICEConnectionState))
	OnConnectionStateChange(func(webrtc.PeerConnectionState))
	OnTrack(func(TrackRemote, RTPReceiver))
	OnDataChannel(func(*webrtc.DataChannel))
	OnNegotiationNeeded(func())
	WriteRTCP(pkts []rtcp.Packet) error
	ICEGatheringState() webrtc.ICEGatheringState
	LocalDescription() *webrtc.SessionDescription
}

type RTPTransceiver interface {
}

type TrackRemote interface {
	Read(p []byte) (n int, attrs interceptor.Attributes, err error)
	Kind() webrtc.RTPCodecType
	PayloadType() webrtc.PayloadType
	Codec() webrtc.RTPCodecParameters
	SSRC() webrtc.SSRC
}

type RTPReceiver interface {
}

func GatheringCompletePromise(pc PeerConnection) <-chan struct{} {
	recorder, ok := pc.(*RecorderPeerConnection)
	if ok {
		return webrtc.GatheringCompletePromise(recorder.pionpc)
	}
	panic("unknown peer connection type")
}
