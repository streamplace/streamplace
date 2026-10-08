package media

import (
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"sync"
)

type earlyRTPRecorder struct {
	mu       sync.Mutex
	headers  []rtp.Header
	payloads [][]byte
}

func (r *earlyRTPRecorder) WriteRTP(h *rtp.Header, p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.headers = append(r.headers, *h)
	r.payloads = append(r.payloads, append([]byte(nil), p...))
	return len(p), nil
}
func (r *earlyRTPRecorder) Write(p []byte) (int, error) { return len(p), nil }

type earlyTrackContext struct {
	codec    webrtc.RTPCodecParameters
	recorder *earlyRTPRecorder
}

func (c earlyTrackContext) CodecParameters() []webrtc.RTPCodecParameters {
	return []webrtc.RTPCodecParameters{c.codec}
}
func (c earlyTrackContext) HeaderExtensions() []webrtc.RTPHeaderExtensionParameter { return nil }
func (c earlyTrackContext) SSRC() webrtc.SSRC                                      { return 1 }
func (c earlyTrackContext) SSRCRetransmission() webrtc.SSRC                        { return 0 }
func (c earlyTrackContext) SSRCForwardErrorCorrection() webrtc.SSRC                { return 0 }
func (c earlyTrackContext) WriteStream() webrtc.TrackLocalWriter                   { return c.recorder }
func (c earlyTrackContext) ID() string                                             { return "gate" }
func (c earlyTrackContext) RTCPReader() interceptor.RTCPReader                     { return nil }
