package media

import (
	"fmt"
	"strings"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
)

// newWebRTCAPI builds the pion API + configuration Streamplace uses for WebRTC
// sessions in one video codec, video (H.264 or AV1), with Opus audio, default
// interceptors plus an interval PLI so the publisher keeps sending keyframes.
// The isolated WHIP worker builds its own, since it owns the PeerConnection in
// its own process.
//
// One video codec per API, because a session carries one, and because pion
// negotiates only the codecs that match an offer exactly when any does: H.264,
// registered without fmtp, only ever matches partially, so an API with AV1
// too would drop H.264 from every offer that has AV1 in it.
func newWebRTCAPI(video VideoCodec) (*webrtc.API, webrtc.Configuration, error) {
	m := &webrtc.MediaEngine{}
	i := &interceptor.Registry{}

	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, webrtc.Configuration{}, fmt.Errorf("failed to create intervalpli factory: %w", err)
	}
	i.Add(intervalPliFactory)

	payloadType := webrtc.PayloadType(102)
	if video.Name == VideoAV1.Name {
		payloadType = 45
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: video.MimeType, ClockRate: 90000},
		PayloadType:        payloadType,
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

// newWebRTCAPIs builds MakeMediaManager's APIs (see newWebRTCAPI), one per
// video codec, by MIME type.
func newWebRTCAPIs() (map[string]*webrtc.API, webrtc.Configuration, error) {
	apis := map[string]*webrtc.API{}
	var config webrtc.Configuration
	for _, c := range []VideoCodec{VideoH264, VideoAV1} {
		api, cfg, err := newWebRTCAPI(c)
		if err != nil {
			return nil, webrtc.Configuration{}, err
		}
		apis[c.MimeType], config = api, cfg
	}
	return apis, config, nil
}

// ingestVideoCodec is the video codec of a WHIP publisher's session: H.264
// whenever its offer has it, else AV1 if offered. A publisher offering AV1
// merely as an option must keep sending H.264, or viewers that can't decode
// AV1 (iOS) lose the stream; a publisher offering only AV1 gets AV1.
func ingestVideoCodec(offer *webrtc.SessionDescription) VideoCodec {
	offered := offeredVideoMimeTypes(offer)
	if !offered[strings.ToLower(VideoH264.MimeType)] && offered[strings.ToLower(VideoAV1.MimeType)] {
		return VideoAV1
	}
	return VideoH264
}

// offeredVideoMimeTypes lists the video codecs a session description takes, by
// lowercased MIME type ("video/av1"). Empty when the offer can't be read
// (SetRemoteDescription reports why).
func offeredVideoMimeTypes(offer *webrtc.SessionDescription) map[string]bool {
	offered := map[string]bool{}
	parsed, err := offer.Unmarshal()
	if err != nil {
		return offered
	}
	for _, md := range parsed.MediaDescriptions {
		if md.MediaName.Media != "video" {
			continue
		}
		for _, a := range md.Attributes {
			// rtpmap:<payload type> <encoding name>/<clock rate>
			if _, enc, ok := strings.Cut(a.Value, " "); ok && a.Key == "rtpmap" {
				name, _, _ := strings.Cut(enc, "/")
				offered["video/"+strings.ToLower(name)] = true
			}
		}
	}
	return offered
}
