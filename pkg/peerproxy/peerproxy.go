package peerproxy

import (
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
	OnTrack(func(*webrtc.TrackRemote, *webrtc.RTPReceiver))
	OnDataChannel(func(*webrtc.DataChannel))
	OnNegotiationNeeded(func())
	WriteRTCP(pkts []rtcp.Packet) error
	ICEGatheringState() webrtc.ICEGatheringState
	LocalDescription() *webrtc.SessionDescription
}

type RTPTransceiver interface {
}

func TranscieverPlease() RTPTransceiver {
	return &webrtc.RTPTransceiver{}
}

func TranscieverThankYou() *webrtc.RTPTransceiver {
	t, ok := TranscieverPlease().(*webrtc.RTPTransceiver)
	if !ok {
		panic("TranscieverPlease() is not a webrtc.RTPTransceiver")
	}
	return t
}
