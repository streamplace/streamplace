package rtcrec

import (
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type RecordingTrackRemote struct {
	track  *webrtc.TrackRemote
	stream *RecorderStream
	pc     *RecordingPeerConnection
}

func (t *RecordingTrackRemote) do(f func()) {
	go f()
}

func (t *RecordingTrackRemote) Read(p []byte) (n int, attrs interceptor.Attributes, err error) {
	n, attrs, err = t.track.Read(p)
	now := time.Now()
	b2 := make([]byte, n)
	copy(b2, p)
	t.pc.Do(func() {
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
	})
	return n, attrs, err
}

func (t *RecordingTrackRemote) Codec() webrtc.RTPCodecParameters {
	now := time.Now()
	codec := t.track.Codec()
	t.pc.Do(func() {
		t.stream.Event(WebRTCEvent{
			TrackCodec: &TrackCodec{
				SSRC:  t.track.SSRC(),
				Codec: codec,
			},
			Time: now,
		})
	})
	return codec
}

func (t *RecordingTrackRemote) ID() string {
	return t.track.ID()
}

func (t *RecordingTrackRemote) Kind() webrtc.RTPCodecType {
	now := time.Now()
	kind := t.track.Kind()
	t.pc.Do(func() {
		t.stream.Event(WebRTCEvent{
			TrackKind: &TrackKind{
				SSRC: t.track.SSRC(),
				Kind: kind,
			},
			Time: now,
		})
	})
	return kind
}

func (t *RecordingTrackRemote) PayloadType() webrtc.PayloadType {
	now := time.Now()
	payloadType := t.track.PayloadType()
	t.pc.Do(func() {
		t.stream.Event(WebRTCEvent{
			TrackPayloadType: &TrackPayloadType{
				SSRC:        t.track.SSRC(),
				PayloadType: payloadType,
			},
			Time: now,
		})
	})
	return payloadType
}

func (t *RecordingTrackRemote) SSRC() webrtc.SSRC {
	now := time.Now()
	ssrc := t.track.SSRC()
	t.pc.Do(func() {
		t.stream.Event(WebRTCEvent{
			Time: now,
			TrackSSRC: &TrackSSRC{
				SSRC: ssrc,
			},
		})
	})
	return ssrc
}
