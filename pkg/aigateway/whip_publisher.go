package aigateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"stream.place/streamplace/pkg/log"
)

const (
	// whipICEGatherTimeout is how long to wait for ICE candidate gathering.
	whipICEGatherTimeout = 3 * time.Second

	// whipDeleteTimeout is how long to wait when sending DELETE to clean up session.
	whipDeleteTimeout = 2 * time.Second

	// defaultVideoDuration is the default sample duration for video if not specified.
	defaultVideoDuration = 33 * time.Millisecond

	// defaultAudioDuration is the default sample duration for audio if not specified.
	defaultAudioDuration = 20 * time.Millisecond
)

// WHIPPublisher publishes media to a WHIP endpoint using WebRTC.
// It supports H.264 video and Opus audio tracks.
type WHIPPublisher struct {
	ctx    context.Context
	cancel context.CancelFunc

	whipURL string
	deleteURL string

	startedMu sync.Mutex
	started   bool

	pc         *webrtc.PeerConnection
	audioTrack *webrtc.TrackLocalStaticSample
	videoTrack *webrtc.TrackLocalStaticSample
}

// NewWHIPPublisher creates a new WHIP publisher for the given endpoint URL.
func NewWHIPPublisher(ctx context.Context, whipURL string) *WHIPPublisher {
	ctx, cancel := context.WithCancel(ctx)
	return &WHIPPublisher{ctx: ctx, cancel: cancel, whipURL: whipURL}
}

// Start establishes the WebRTC connection to the WHIP endpoint.
// It creates audio and video tracks and performs the WHIP handshake.
func (p *WHIPPublisher) Start() error {
	p.startedMu.Lock()
	defer p.startedMu.Unlock()

	if p.started {
		return nil
	}
	if p.whipURL == "" {
		return fmt.Errorf("empty whipURL")
	}

	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		return fmt.Errorf("register default codecs: %w", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(me))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		return fmt.Errorf("new peer connection: %w", err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "streamplace")
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("new audio track: %w", err)
	}
	audioSender, err := pc.AddTrack(audioTrack)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("add audio track: %w", err)
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "streamplace")
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("new video track: %w", err)
	}
	videoSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("add video track: %w", err)
	}

	go p.discardRTCP(audioSender)
	go p.discardRTCP(videoSender)

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("create offer: %w", err)
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		_ = pc.Close()
		return fmt.Errorf("set local description: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
	case <-time.After(whipICEGatherTimeout):
	case <-p.ctx.Done():
		_ = pc.Close()
		return p.ctx.Err()
	}

	local := pc.LocalDescription()
	if local == nil {
		_ = pc.Close()
		return fmt.Errorf("missing local description")
	}

	req, err := http.NewRequestWithContext(p.ctx, http.MethodPost, p.whipURL, strings.NewReader(local.SDP))
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("new WHIP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/sdp")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("do WHIP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		_ = pc.Close()
		return fmt.Errorf("WHIP request failed: %s: %s", resp.Status, string(b))
	}

	answerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("read WHIP answer: %w", err)
	}

	location := resp.Header.Get("Location")
	if location != "" {
		p.deleteURL = resolveWHIPDeleteURL(p.whipURL, location)
	}

	answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(answerBytes)}
	if err := pc.SetRemoteDescription(answer); err != nil {
		_ = pc.Close()
		return fmt.Errorf("set remote description: %w", err)
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Debug(p.ctx, "WHIP ICE state", "state", state.String())
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected || state == webrtc.ICEConnectionStateClosed {
			p.cancel()
		}
	})

	go func() {
		<-p.ctx.Done()
		_ = pc.Close()
	}()

	p.pc = pc
	p.audioTrack = audioTrack
	p.videoTrack = videoTrack
	p.started = true

	log.Log(p.ctx, "WHIP publisher started", "whipURL", p.whipURL)
	return nil
}

func (p *WHIPPublisher) discardRTCP(sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		_, _, err := sender.Read(buf)
		if err != nil {
			return
		}
	}
}

// WriteVideoSample sends a video sample to the WHIP endpoint.
// If duration is zero or negative, a default duration is used.
func (p *WHIPPublisher) WriteVideoSample(data []byte, duration time.Duration) error {
	if duration <= 0 {
		duration = defaultVideoDuration
	}
	p.startedMu.Lock()
	track := p.videoTrack
	started := p.started
	p.startedMu.Unlock()
	if !started || track == nil {
		return nil
	}
	return track.WriteSample(pionmedia.Sample{Data: data, Duration: duration})
}

// WriteAudioSample sends an audio sample to the WHIP endpoint.
// If duration is zero or negative, a default duration is used.
func (p *WHIPPublisher) WriteAudioSample(data []byte, duration time.Duration) error {
	if duration <= 0 {
		duration = defaultAudioDuration
	}
	p.startedMu.Lock()
	track := p.audioTrack
	started := p.started
	p.startedMu.Unlock()
	if !started || track == nil {
		return nil
	}
	return track.WriteSample(pionmedia.Sample{Data: data, Duration: duration})
}

// Stop closes the WebRTC connection and sends a DELETE request to clean up the session.
func (p *WHIPPublisher) Stop() {
	p.startedMu.Lock()
	deleteURL := p.deleteURL
	p.started = false
	p.startedMu.Unlock()

	if deleteURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), whipDeleteTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		if err == nil {
			_, _ = http.DefaultClient.Do(req)
		}
		cancel()
	}

	p.cancel()
}

func resolveWHIPDeleteURL(whipURL, location string) string {
	if location == "" {
		return ""
	}
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return location
	}
	if !strings.HasPrefix(location, "/") {
		base, err := url.Parse(whipURL)
		if err != nil {
			return location
		}
		ref, err := url.Parse(location)
		if err != nil {
			return location
		}
		return base.ResolveReference(ref).String()
	}

	base := strings.TrimRight(whipURL, "/")
	if idx := strings.Index(base, "/whip"); idx > 0 {
		base = base[:idx]
	}
	return base + location
}
