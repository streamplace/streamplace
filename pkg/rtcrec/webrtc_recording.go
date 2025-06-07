package rtcrec

import (
	"io"
	"time"

	"github.com/fxamacker/cbor/v2"
)

type WebRTCRecording struct {
	Events []WebRTCEvent
}

type WebRTCEvent struct {
	Offer  *Offer    `json:"offer,omitempty"`
	Answer *Answer   `json:"answer,omitempty"`
	Time   time.Time `json:"time"`
}

func (e *WebRTCEvent) Detail() WebRTCEventDetail {
	if e.Offer != nil {
		return e.Offer
	}
	if e.Answer != nil {
		return e.Answer
	}
	return nil
}

type WebRTCEventDetail interface{}

type Offer struct {
	SDPOffer string `json:"offer"`
}

type Answer struct {
	SDPAnswer string `json:"answer"`
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

func (s *RecorderStream) Event(event WebRTCEvent) error {
	return s.encoder.Encode(event)
}
