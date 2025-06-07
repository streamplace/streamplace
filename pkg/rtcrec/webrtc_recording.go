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
	Offer  *OfferEvent
	Answer *AnswerEvent
	Time   time.Time
}

type OfferEvent struct {
	Offer string
}

type AnswerEvent struct {
	Answer string
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
	event.Time = time.Now()
	return s.encoder.Encode(event)
}
