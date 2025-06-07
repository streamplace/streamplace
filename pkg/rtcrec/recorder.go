package rtcrec

import (
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type RecorderPeerConnection struct {
	pionpc *webrtc.PeerConnection
}

func NewRecorderPeerConnection(pionpc *webrtc.PeerConnection) PeerConnection {
	return &RecorderPeerConnection{
		pionpc: pionpc,
	}
}

func (pc *RecorderPeerConnection) Close() error {
	return pc.pionpc.Close()
}

func (pc *RecorderPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	return pc.pionpc.CreateAnswer(options)
}

func (pc *RecorderPeerConnection) CreateOffer(options *webrtc.OfferOptions) (webrtc.SessionDescription, error) {
	return pc.pionpc.CreateOffer(options)
}

func (pc *RecorderPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	return pc.pionpc.SetLocalDescription(desc)
}

func (pc *RecorderPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return pc.pionpc.SetRemoteDescription(desc)
}

func (pc *RecorderPeerConnection) LocalDescription() *webrtc.SessionDescription {
	return pc.pionpc.LocalDescription()
}

func (pc *RecorderPeerConnection) RemoteDescription() *webrtc.SessionDescription {
	return pc.pionpc.RemoteDescription()
}

func (pc *RecorderPeerConnection) OnICEConnectionStateChange(f func(webrtc.ICEConnectionState)) {
	pc.pionpc.OnICEConnectionStateChange(f)
}

func (pc *RecorderPeerConnection) OnConnectionStateChange(f func(webrtc.PeerConnectionState)) {
	pc.pionpc.OnConnectionStateChange(f)
}

func (pc *RecorderPeerConnection) OnTrack(f func(TrackRemote, RTPReceiver)) {
	pc.pionpc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		f(track, receiver)
	})
}

func (pc *RecorderPeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	return pc.pionpc.WriteRTCP(pkts)
}

func (pc *RecorderPeerConnection) AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error) {
	return pc.pionpc.AddTransceiverFromKind(kind, init...)
}

func (pc *RecorderPeerConnection) ICEGatheringState() webrtc.ICEGatheringState {
	return pc.pionpc.ICEGatheringState()
}

func (pc *RecorderPeerConnection) OnDataChannel(f func(*webrtc.DataChannel)) {
	pc.pionpc.OnDataChannel(f)
}

func (pc *RecorderPeerConnection) OnNegotiationNeeded(f func()) {
	pc.pionpc.OnNegotiationNeeded(f)
}
