package rtcrec

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/log"
)

type WebRTCRecording struct {
	Events []WebRTCEvent `json:"events,omitempty"`
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
	TrackRead                *TrackRead                `json:"trackRead,omitempty"`
	TrackCodec               *TrackCodec               `json:"trackCodec,omitempty"`
	TrackKind                *TrackKind                `json:"trackKind,omitempty"`
	TrackPayloadType         *TrackPayloadType         `json:"trackPayloadType,omitempty"`
	TrackSSRC                *TrackSSRC                `json:"trackSSRC,omitempty"`
	AddTransceiverFromKind   *AddTransceiverFromKind   `json:"addTransceiverFromKind,omitempty"`
	ICEGatheringState        *ICEGatheringState        `json:"iceGatheringState,omitempty"`
	DataChannel              *DataChannel              `json:"dataChannel,omitempty"`
	NegotiationNeeded        *NegotiationNeeded        `json:"negotiationNeeded,omitempty"`
	Time                     time.Time                 `json:"time,omitempty"`
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
	SDPOffer string `json:"sdpOffer,omitempty"`
}

type CreateAnswer struct {
	SDPAnswer string `json:"sdpAnswer,omitempty"`
}

type SetRemoteDescription struct {
	SDPRemoteDescription string `json:"sdpRemoteDescription,omitempty"`
}

type SetLocalDescription struct {
	SDPLocalDescription string `json:"sdpRemoteDescription,omitempty"`
}

type LocalDescription struct {
	SDPLocalDescription string `json:"sdpLocalDescription,omitempty"`
}

type ICEConnectionStateChange struct {
	ICEConnectionState webrtc.ICEConnectionState `json:"iceConnectionState,omitempty"`
}

type ConnectionStateChange struct {
	ConnectionState webrtc.PeerConnectionState `json:"connectionState,omitempty"`
}

type Track struct {
	ID          string              `json:"id,omitempty"`
	Kind        webrtc.RTPCodecType `json:"kind,omitempty"`
	SSRC        webrtc.SSRC         `json:"ssrc,omitempty"`
	PayloadType webrtc.PayloadType  `json:"payloadType,omitempty"`
	StreamID    string              `json:"streamId,omitempty"`
	Msid        string              `json:"msid,omitempty"`
	RID         string              `json:"rid,omitempty"`
}

type TrackRead struct {
	SSRC  webrtc.SSRC `json:"ssrc,omitempty"`
	Data  []byte      `json:"data,omitempty"`
	Count int         `json:"count,omitempty"`
	Err   string      `json:"err,omitempty"`
}

type TrackCodec struct {
	SSRC  webrtc.SSRC               `json:"ssrc,omitempty"`
	Codec webrtc.RTPCodecParameters `json:"codec,omitempty"`
}

type TrackKind struct {
	SSRC webrtc.SSRC         `json:"ssrc,omitempty"`
	Kind webrtc.RTPCodecType `json:"kind,omitempty"`
}

type TrackPayloadType struct {
	SSRC        webrtc.SSRC        `json:"ssrc,omitempty"`
	PayloadType webrtc.PayloadType `json:"payloadType,omitempty"`
}

type TrackSSRC struct {
	SSRC webrtc.SSRC `json:"ssrc,omitempty"`
}

type RecorderStream struct {
	encoder *cbor.Encoder
}

func NewRecorderStream(w io.Writer) (*RecorderStream, error) {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeRFC3339Nano
	em, err := opts.EncMode()
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder mode: %w", err)
	}
	encoder := em.NewEncoder(w)

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

type AddTransceiverFromKind struct {
	Kind webrtc.RTPCodecType `json:"kind,omitempty"`
}

type ICEGatheringState struct {
	State webrtc.ICEGatheringState `json:"state,omitempty"`
}

type DataChannel struct {
	Label string `json:"label,omitempty"`
}

type NegotiationNeeded struct{}
