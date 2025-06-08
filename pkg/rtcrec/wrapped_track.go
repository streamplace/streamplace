package rtcrec

import (
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type WrappedTrackRemote struct {
	track  *webrtc.TrackRemote
	stream *RecorderStream
}

func (t *WrappedTrackRemote) Read(p []byte) (n int, attrs interceptor.Attributes, err error) {
	n, attrs, err = t.track.Read(p)
	now := time.Now()
	b2 := make([]byte, n)
	copy(b2, p)
	go func() {
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		t.stream.Event(WebRTCEvent{
			TrackRead: &TrackRead{
				Data:  b2,
				SSRC:  t.track.SSRC(),
				Count: n,
				// Attrs:   attrs,
				Err: errString,
			},
			Time: now,
		})
	}()
	return n, attrs, err
}

func (t *WrappedTrackRemote) Codec() webrtc.RTPCodecParameters {
	now := time.Now()
	codec := t.track.Codec()
	go func() {
		t.stream.Event(WebRTCEvent{
			TrackCodec: &TrackCodec{
				SSRC:  t.track.SSRC(),
				Codec: codec,
			},
			Time: now,
		})
	}()
	return codec
}

func (t *WrappedTrackRemote) ID() string {
	return t.track.ID()
}

func (t *WrappedTrackRemote) Kind() webrtc.RTPCodecType {
	now := time.Now()
	kind := t.track.Kind()
	go func() {
		t.stream.Event(WebRTCEvent{
			TrackKind: &TrackKind{
				SSRC: t.track.SSRC(),
				Kind: kind,
			},
			Time: now,
		})
	}()
	return kind
}

func (t *WrappedTrackRemote) PayloadType() webrtc.PayloadType {
	now := time.Now()
	payloadType := t.track.PayloadType()
	go func() {
		t.stream.Event(WebRTCEvent{
			TrackPayloadType: &TrackPayloadType{
				SSRC:        t.track.SSRC(),
				PayloadType: payloadType,
			},
			Time: now,
		})
	}()
	return payloadType
}

func (t *WrappedTrackRemote) SSRC() webrtc.SSRC {
	now := time.Now()
	ssrc := t.track.SSRC()
	go func() {
		t.stream.Event(WebRTCEvent{
			Time: now,
			TrackSSRC: &TrackSSRC{
				SSRC: ssrc,
			},
		})
	}()
	return ssrc
}
