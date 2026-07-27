package media

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/renditions"
)

const renditionDataChannelLabel = "rendition"

type renditionSwap struct {
	name string
	ack  chan error
}

// validRenditionName reports whether name is a rendition the playback path
// could actually be producing for a stream: "source" or one of the
// configured transcoded rendition names. This is defense-in-depth against
// typos or arbitrary strings sent over the data channel, which would
// otherwise subscribe to a bus key that never receives segments and stall
// playback silently.
func validRenditionName(name string) bool {
	if name == "source" {
		return true
	}
	for _, r := range renditions.DesiredRenditions {
		if r.Name == name {
			return true
		}
	}
	return false
}

// handleRenditionSwapRequest processes one client's request (over the playback
// data channel) to switch the rendition being forwarded. Replies on the same
// channel with {"rendition": ..., "applied": true} or {"error": ...}.
func handleRenditionSwapRequest(ctx context.Context, dc *webrtc.DataChannel, data []byte, audioOnly bool, swapCh chan<- renditionSwap) {
	reply := func(v any) {
		bs, err := json.Marshal(v)
		if err != nil {
			return
		}
		if err := dc.SendText(string(bs)); err != nil {
			log.Warn(ctx, "could not send rendition swap reply", "error", err)
		}
	}
	var req struct {
		Rendition string `json:"rendition"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		reply(map[string]string{"error": fmt.Sprintf("invalid rendition request: %s", err)})
		return
	}
	if audioOnly {
		reply(map[string]string{"error": "cannot switch rendition on an audio-only session"})
		return
	}
	if req.Rendition == "" || req.Rendition == renditions.AudioRendition.Name {
		reply(map[string]string{"error": fmt.Sprintf("invalid rendition %q", req.Rendition)})
		return
	}
	if !validRenditionName(req.Rendition) {
		reply(map[string]string{"error": fmt.Sprintf("unknown rendition %q", req.Rendition)})
		return
	}
	ack := make(chan error, 1)
	select {
	case swapCh <- renditionSwap{name: req.Rendition, ack: ack}:
	case <-ctx.Done():
		return
	}
	select {
	case err := <-ack:
		if err != nil {
			reply(map[string]string{"error": err.Error()})
		} else {
			reply(map[string]any{"rendition": req.Rendition, "applied": true})
		}
	case <-time.After(5 * time.Second):
		reply(map[string]string{"error": "rendition switch timed out"})
	case <-ctx.Done():
	}
}

// pumpSegments forwards packetized segments from the bus to out for the
// duration of the context, starting with rendition `initial`. Requests on
// swapCh switch the subscription to another rendition at the next segment
// boundary — every rendition segment starts on a keyframe, so the receiver's
// decoder sees a clean splice on the same RTP stream. latency counts media
// enqueued but not yet played; the writer goroutine decrements it.
func pumpSegments(ctx context.Context, mm *MediaManager, user string, initial string, viewer string, swapCh <-chan renditionSwap, latency *atomic.Int64, out chan<- *bus.PacketizedSegment) {
	current := initial
	// buffer 2 on join so playback can start instantly; on swap we want the
	// live edge only, not the new rendition's recent history
	segChan := mm.bus.SubscribeSegmentBuf(ctx, user, current, 2)
	defer func() { mm.bus.UnsubscribeSegment(ctx, user, current, segChan) }()
	forward := func(file *bus.Seg) bool {
		if !file.Published && viewer != user && !mm.cli.WideOpen {
			log.Warn(ctx, "segment is not published and viewer is not the user", "viewer", viewer, "user", user)
			return true
		}
		latency.Add(int64(file.PacketizedData.Duration))
		select {
		case out <- file.PacketizedData:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for {
		select {
		case <-ctx.Done():
			log.Debug(ctx, "exiting segment reader")
			return
		case sw := <-swapCh:
			mm.bus.UnsubscribeSegment(ctx, user, current, segChan)
			// forward any in-flight segment of the old rendition so playback
			// stays continuous across the splice
			select {
			case file := <-segChan.C:
				if !forward(file) {
					sw.ack <- ctx.Err()
					return
				}
			default:
			}
			current = sw.name
			segChan = mm.bus.SubscribeSegmentBuf(ctx, user, current, 0)
			sw.ack <- nil
			log.Log(ctx, "switched playback rendition", "user", user, "rendition", sw.name)
		case file := <-segChan.C:
			if !forward(file) {
				return
			}
		}
	}
}

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCPlayback2(ctx context.Context, user string, rendition string, offer *webrtc.SessionDescription, viewer string) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx = log.WithLogValues(ctx, "mediafunc", "WebRTCPlayback")

	// Create a new RTCPeerConnection
	peerConnection, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC peer connection: %w", err)
	}

	audioOnly := rendition == renditions.AudioRendition.Name

	var videoTrack *webrtc.TrackLocalStaticSample
	var videoRTPSender *webrtc.RTPSender
	if !audioOnly {
		videoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
		if err != nil {
			return nil, fmt.Errorf("failed to create video track: %w", err)
		}
		videoRTPSender, err = peerConnection.AddTrack(videoTrack)
		if err != nil {
			return nil, fmt.Errorf("failed to add video track to peer connection: %w", err)
		}
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}
	audioRTPSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add audio track to peer connection: %w", err)
	}

	close := func() {
		if cErr := peerConnection.Close(); cErr != nil {
			log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
		}
	}

	// In-session rendition switching: the client opens a "rendition" data
	// channel; requests on it are applied by the segment reader goroutine.
	swapCh := make(chan renditionSwap, 1)
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() != renditionDataChannelLabel {
			return
		}
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			handleRenditionSwapRequest(ctx, dc, msg.Data, audioOnly, swapCh)
		})
	})

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(*offer); err != nil {
		close()
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		close()
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		close()
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Setup complete! Now we boot up streaming in the background while returning the SDP offer to the user.

	// The session only counts as a viewer once the peer connection actually
	// establishes — counting at SDP-answer time inflated the count with
	// handshakes that never connected (each lingering until ICE failure
	// detection). The mutex pairs the increment with exactly one decrement
	// even if a connect races session teardown.
	var viewerMu sync.Mutex
	viewerCounted := false
	viewerDone := false
	markConnected := func() {
		viewerMu.Lock()
		defer viewerMu.Unlock()
		if viewerDone || viewerCounted {
			return
		}
		viewerCounted = true
		mm.IncrementViewerCount(user, "webrtc")
	}
	markDone := func() {
		viewerMu.Lock()
		defer viewerMu.Unlock()
		viewerDone = true
		if viewerCounted {
			viewerCounted = false
			mm.DecrementViewerCount(user, "webrtc")
		}
	}

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer markDone()

		var latency atomic.Int64

		packetQueue := make(chan *bus.PacketizedSegment, 1024)
		busRendition := rendition
		if audioOnly {
			busRendition = "source"
		}
		go pumpSegments(ctx, mm, user, busRendition, viewer, swapCh, &latency, packetQueue)

		go func() {
			go func() {
				<-ctx.Done()
				if cErr := peerConnection.Close(); cErr != nil {
					log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
				}
			}()

			var scalar float64 = 1

			for {
				select {
				case <-ctx.Done():
					return
				case packet := <-packetQueue:
					latency.Add(-int64(packet.Duration))
					scalar = getPlaybackRate(time.Duration(latency.Load()))
					log.Debug(ctx, "playback latency", "latency", time.Duration(latency.Load()), "scalar", scalar)
					var videoDur time.Duration
					var audioDur time.Duration
					if len(packet.Video) > 0 {
						videoDur = packet.Duration / time.Duration(len(packet.Video))
					}
					if len(packet.Audio) > 0 {
						audioDur = packet.Duration / time.Duration(len(packet.Audio))
					}
					g, _ := errgroup.WithContext(ctx)

					if !audioOnly && videoDur > 0 {
						g.Go(func() error {
							ticker := time.NewTicker(time.Duration(float64(videoDur) * (1 / scalar)))
							defer ticker.Stop()
							for _, video := range packet.Video {
								err := videoTrack.WriteSample(media.Sample{Data: video, Duration: videoDur})
								if err != nil {
									return fmt.Errorf("failed to write video sample: %w", err)
								}

								select {
								case <-ctx.Done():
									return nil
								case <-ticker.C:
									continue
								}
							}
							return nil
						})
					} else if !audioOnly {
						log.Warn(ctx, "no video samples to write")
					}
					if audioDur > 0 {
						g.Go(func() error {
							ticker := time.NewTicker(time.Duration(float64(audioDur) * (1 / scalar)))
							defer ticker.Stop()
							for _, audio := range packet.Audio {
								err := audioTrack.WriteSample(media.Sample{Data: audio, Duration: audioDur})
								if err != nil {
									return fmt.Errorf("failed to write audio sample: %w", err)
								}
								select {
								case <-ctx.Done():
									return nil
								case <-ticker.C:
									continue
								}
							}
							return nil
						})

						if err := g.Wait(); err != nil {
							log.Error(ctx, "failed to write samples", "error", err)
							cancel()
						}
					} else {
						log.Warn(ctx, "no audio samples to write")
					}
				}
			}
		}()

		if !audioOnly {
			go func() {
				rtcpBuf := make([]byte, 1500)
				for {
					if _, _, rtcpErr := videoRTPSender.Read(rtcpBuf); rtcpErr != nil {
						return
					}
				}
			}()
		}

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := audioRTPSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		// Set the handler for ICE connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			log.Log(ctx, "Connection State has changed", "state", connectionState.String())
		})

		// Set the handler for Peer connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
			log.Log(ctx, "Peer Connection State has changed", "state", s.String())

			if s == webrtc.PeerConnectionStateConnected {
				markConnected()
			}

			if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateDisconnected {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has gone to failed, exiting")
				cancel()
			}
		})

		<-ctx.Done()

		log.Warn(ctx, "exiting playback")

	}()
	select {
	case <-gatherComplete:
		return peerConnection.LocalDescription(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// getPlaybackRate returns a playback rate that eases from 1.0 to 1.5 between 7 and 60 seconds
func getPlaybackRate(dur time.Duration) float64 {
	switch {
	case dur <= 7*time.Second:
		return 1.0
	case dur >= 60*time.Second:
		return 1.5
	default:
		// Linear interpolation between (7,1.0) and (60,1.5)
		progress := (float64(dur) - float64(7*time.Second)) / (float64(60*time.Second) - float64(7*time.Second))
		return 1.0 + (0.5 * progress)
	}
}
