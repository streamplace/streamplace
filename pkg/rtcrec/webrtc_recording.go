package rtcrec

import (
	"context"
	"io"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/log"
)

type WebRTCRecording struct {
	Events []WebRTCEvent
}

type WebRTCEvent struct {
	Offer                    *Offer                    `json:"offer,omitempty"`
	CreateAnswer             *CreateAnswer             `json:"answer,omitempty"`
	SetRemoteDescription     *SetRemoteDescription     `json:"setRemoteDescription,omitempty"`
	SetLocalDescription      *SetLocalDescription      `json:"setLocalDescription,omitempty"`
	LocalDescription         *LocalDescription         `json:"localDescription,omitempty"`
	ICEConnectionStateChange *ICEConnectionStateChange `json:"iceConnectionStateChange,omitempty"`
	ConnectionStateChange    *ConnectionStateChange    `json:"connectionStateChange,omitempty"`
	Track                    *Track                    `json:"track,omitempty"`
	Time                     time.Time                 `json:"time"`
}

func (e *WebRTCEvent) Detail() WebRTCEventDetail {
	if e.Offer != nil {
		return e.Offer
	}
	if e.CreateAnswer != nil {
		return e.CreateAnswer
	}
	return nil
}

type WebRTCEventDetail interface{}

type Offer struct {
	SDPOffer string `json:"sdpOffer"`
}

type CreateAnswer struct {
	SDPAnswer string `json:"sdpAnswer"`
}

type SetRemoteDescription struct {
	SDPRemoteDescription string `json:"sdpRemoteDescription"`
}

type SetLocalDescription struct {
	SDPLocalDescription string `json:"sdpRemoteDescription"`
}

type LocalDescription struct {
	SDPLocalDescription string `json:"sdpLocalDescription"`
}

type ICEConnectionStateChange struct {
	ICEConnectionState webrtc.ICEConnectionState `json:"iceConnectionState"`
}

type ConnectionStateChange struct {
	ConnectionState webrtc.PeerConnectionState `json:"connectionState"`
}

type Track struct {
	Track *webrtc.TrackRemote `json:"track"`
}

type RecorderStream struct {
	encoder *cbor.Encoder
}

func NewRecorderStream(w io.Writer) (*RecorderStream, error) {
	encoder := cbor.NewEncoder(w)

	return &RecorderStream{
		encoder: encoder,
	}, nil
}

func (s *RecorderStream) Event(event WebRTCEvent) {
	err := s.encoder.Encode(event)
	if err != nil {
		log.Log(context.Background(), "error encoding event", "error", err)
	}
}
