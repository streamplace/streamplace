package rtcrec

import (
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type WrappedPeerConnection struct {
	pionpc *webrtc.PeerConnection
}

func NewWrappedPC(pionpc *webrtc.PeerConnection) PeerConnection {
	return &WrappedPeerConnection{
		pionpc: pionpc,
	}
}

func (pc *WrappedPeerConnection) Close() error {
	return pc.pionpc.Close()
}

func (pc *WrappedPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	return pc.pionpc.CreateAnswer(options)
}

func (pc *WrappedPeerConnection) CreateOffer(options *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	return pc.pionpc.CreateOffer(options)
}

func (pc *WrappedPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	return pc.pionpc.SetLocalDescription(desc)
}

func (pc *WrappedPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return pc.pionpc.SetRemoteDescription(desc)
}

func (pc *WrappedPeerConnection) LocalDescription() *webrtc.SessionDescription {
	return pc.pionpc.LocalDescription()
}

func (pc *WrappedPeerConnection) RemoteDescription() *webrtc.SessionDescription {
	return pc.pionpc.RemoteDescription()
}

func (pc *WrappedPeerConnection) OnICEConnectionStateChange(f func(webrtc.ICEConnectionState)) {
	pc.pionpc.OnICEConnectionStateChange(f)
}

func (pc *WrappedPeerConnection) OnConnectionStateChange(f func(webrtc.PeerConnectionState)) {
	pc.pionpc.OnConnectionStateChange(f)
}

func (pc *WrappedPeerConnection) OnTrack(f func(TrackRemote, RTPReceiver)) {
	pc.pionpc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		f(track, receiver)
	})
}

func (pc *WrappedPeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	return pc.pionpc.WriteRTCP(pkts)
}

func (pc *WrappedPeerConnection) AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error) {
	return pc.pionpc.AddTransceiverFromKind(kind, init...)
}

func (pc *WrappedPeerConnection) ICEGatheringState() webrtc.ICEGatheringState {
	return pc.pionpc.ICEGatheringState()
}

func (pc *WrappedPeerConnection) OnDataChannel(f func(*webrtc.DataChannel)) {
	pc.pionpc.OnDataChannel(f)
}

func (pc *WrappedPeerConnection) OnNegotiationNeeded(f func()) {
	pc.pionpc.OnNegotiationNeeded(f)
}
