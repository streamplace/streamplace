package media

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/renditions"
)

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
	busRendition := rendition
	if audioOnly {
		busRendition = "source"
	}

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

	// Audio tracks: the main audio (the full mix) first, then one track per
	// stem group the stream currently carries, "audio-g<group>", ascending.
	// pion binds our audio senders to the offer's audio transceivers in the
	// order we add them, so this order is the client's: it offers one
	// transceiver per track it can take, and we send what fits.
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}
	audioSenders := []*webrtc.RTPSender{}
	audioRTPSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add audio track to peer connection: %w", err)
	}
	audioSenders = append(audioSenders, audioRTPSender)
	stemTracks := map[uint32]*webrtc.TrackLocalStaticSample{}
	for _, group := range stemGroupsFor(mm.bus.LatestSegment(user, busRendition), offeredAudioTracks(offer)-1) {
		track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, fmt.Sprintf("audio-g%d", group), "pion")
		if err != nil {
			return nil, fmt.Errorf("failed to create stem track: %w", err)
		}
		sender, err := peerConnection.AddTrack(track)
		if err != nil {
			return nil, fmt.Errorf("failed to add stem track to peer connection: %w", err)
		}
		audioSenders = append(audioSenders, sender)
		stemTracks[group] = track
	}

	close := func() {
		if cErr := peerConnection.Close(); cErr != nil {
			log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
		}
	}

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

		latency := time.Duration(0)

		packetQueue := make(chan *bus.PacketizedSegment, 1024)
		go func() {
			segChan := mm.bus.SubscribeSegmentBuf(ctx, user, busRendition, 2)
			defer mm.bus.UnsubscribeSegment(ctx, user, busRendition, segChan)
			for {
				select {
				case <-ctx.Done():
					log.Debug(ctx, "exiting segment reader")
					return
				case file := <-segChan.C:
					log.Debug(ctx, "got segment", "file", file.Filepath)
					if !file.Published && viewer != user && !mm.cli.WideOpen {
						log.Warn(ctx, "segment is not published and viewer is not the user", "viewer", viewer, "user", user)
						continue
					}
					latency += file.PacketizedData.Duration
					packetQueue <- file.PacketizedData
				}
			}
		}()

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
					latency -= packet.Duration
					scalar = getPlaybackRate(latency)
					log.Debug(ctx, "playback latency", "latency", latency, "scalar", scalar)
					g, _ := errgroup.WithContext(ctx)
					wroteAny := false

					if !audioOnly && len(packet.Video) > 0 {
						wroteAny = true
						g.Go(func() error {
							return writeSamples(ctx, videoTrack, packet.Video, scalar)
						})
					} else if !audioOnly {
						log.Warn(ctx, "no video samples to write")
					}
					// The audio path deliberately keeps the original
					// uniform-duration ticker instead of writeSamples: Opus
					// packets are a constant 20ms, so the uniform split is
					// exact, and this path is proven in production — audio
					// timing regressions are immediately audible as garbling.
					// Video is the track that needs per-sample durations (its
					// spacing goes non-uniform when an encoder sheds frames).
					// The stems are paced the same way against the same
					// segment duration, so they stay aligned with the mix.
					writeOpus := func(track *webrtc.TrackLocalStaticSample, samples []bus.PacketizedSample) error {
						audioDur := packet.Duration / time.Duration(len(samples))
						ticker := time.NewTicker(time.Duration(float64(audioDur) * (1 / scalar)))
						defer ticker.Stop()
						for _, audio := range samples {
							err := track.WriteSample(media.Sample{Data: audio.Data, Duration: audioDur})
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
					}
					if len(packet.Audio) > 0 && packet.Duration > 0 {
						wroteAny = true
						g.Go(func() error { return writeOpus(audioTrack, packet.Audio) })
					} else {
						log.Warn(ctx, "no audio samples to write")
					}
					for _, stem := range packet.Stems {
						// A stem this session has no track for (it appeared
						// after the offer, or the client offered fewer
						// transceivers) waits for the viewer's next session.
						track := stemTracks[stem.Group]
						if track == nil || len(stem.Audio) == 0 || packet.Duration == 0 {
							continue
						}
						wroteAny = true
						g.Go(func() error { return writeOpus(track, stem.Audio) })
					}
					if wroteAny {
						if err := g.Wait(); err != nil {
							log.Error(ctx, "failed to write samples", "error", err)
							cancel()
						}
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

		for _, sender := range audioSenders {
			go func() {
				rtcpBuf := make([]byte, 1500)
				for {
					if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
						return
					}
				}
			}()
		}

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

// offeredAudioTracks counts the audio transceivers in a viewer's offer: how
// many audio tracks the answer can carry. 1 when the offer can't be read
// (SetRemoteDescription reports why).
func offeredAudioTracks(offer *webrtc.SessionDescription) int {
	parsed, err := offer.Unmarshal()
	if err != nil {
		return 1
	}
	n := 0
	for _, md := range parsed.MediaDescriptions {
		if md.MediaName.Media == "audio" {
			n++
		}
	}
	return max(n, 1)
}

// stemGroupsFor lists the stem groups a playback session sends, ascending:
// the stems of the stream's latest packetized segment (none before the first
// segment, or for a stream without stems), at most limit of them.
func stemGroupsFor(latest *bus.Seg, limit int) []uint32 {
	if latest == nil || latest.PacketizedData == nil || limit <= 0 {
		return nil
	}
	var groups []uint32
	for _, stem := range latest.PacketizedData.Stems {
		if len(groups) == limit {
			break
		}
		groups = append(groups, stem.Group)
	}
	return groups
}

// writeSamples writes one track's samples paced by their real durations —
// the source timeline, so non-uniform frame spacing (bursts and gaps from an
// encoder shedding frames) reaches the viewer intact. The stamped duration
// stays real even while catch-up (scalar > 1) speeds the pacing: the
// receiver's clock is authoritative for playout, sending faster just refills
// its buffer.
//
// Pacing is against a wall-clock schedule, NOT a per-sample sleep: each sleep
// overshoots by scheduler latency, and chaining sleeps accumulates that
// overshoot into real drift (hundreds of ms per segment at high sample rates
// — enough to starve the receiver's jitter buffer). Sleeping until the
// scheduled deadline instead absorbs overshoot in the next iteration, like a
// ticker does.
func writeSamples(ctx context.Context, track *webrtc.TrackLocalStaticSample, samples []bus.PacketizedSample, scalar float64) error {
	start := time.Now()
	var scheduled time.Duration
	for _, s := range samples {
		if err := track.WriteSample(media.Sample{Data: s.Data, Duration: s.Duration}); err != nil {
			return fmt.Errorf("failed to write sample: %w", err)
		}
		scheduled += time.Duration(float64(s.Duration) / scalar)
		wait := scheduled - time.Since(start)
		if wait <= 0 {
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	return nil
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
