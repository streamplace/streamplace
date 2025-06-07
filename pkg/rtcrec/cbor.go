package rtcrec

import (
	"io"

	"github.com/fxamacker/cbor/v2"
)

type WebRTCEventDecoder struct {
	dec *cbor.Decoder
}

func MakeWebRTCDecoder(r io.Reader) (*WebRTCEventDecoder, error) {
	dec := cbor.NewDecoder(r)
	return &WebRTCEventDecoder{dec: dec}, nil
}

func (d *WebRTCEventDecoder) Next() (WebRTCEvent, error) {
	var ev WebRTCEvent
	err := d.dec.Decode(&ev)
	return ev, err
}
