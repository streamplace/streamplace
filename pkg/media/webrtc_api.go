package media

import (
	"fmt"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
)

// newWebRTCAPI builds the pion API + configuration Streamplace uses for WebRTC
// ingest (H264 video + Opus audio, default interceptors plus an interval PLI so
// the publisher keeps sending keyframes). Shared by MakeMediaManager and the
// isolated WHIP worker — the worker builds its own API since it owns the
// PeerConnection in its own process.
func newWebRTCAPI() (*webrtc.API, webrtc.Configuration, error) {
	m := &webrtc.MediaEngine{}
	i := &interceptor.Registry{}

	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, webrtc.Configuration{}, fmt.Errorf("failed to create intervalpli factory: %w", err)
	}
	i.Add(intervalPliFactory)

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		PayloadType:        102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, webrtc.Configuration{}, err
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, webrtc.Configuration{}, err
	}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, webrtc.Configuration{}, fmt.Errorf("failed to register default interceptors: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	return api, config, nil
}
