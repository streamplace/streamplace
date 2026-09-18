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
	"stream.place/streamplace/pkg/spmetrics"
)

func observeWebRTCSegmentLatency(timing *bus.SegmentTiming, streamer, rendition string, at time.Time, stage string) {
	if timing == nil {
		return
	}
	switch stage {
	case "first_rtp", "complete":
	default:
		return
	}
	ingress := timing.Ingress
	switch ingress {
	case "mp4", "rtmp", "whip":
	default:
		ingress = "unknown"
	}
	labels := []string{streamer, ingress, rendition, stage}
	if age := timing.SourceAge(at); age > 0 {
		spmetrics.WebRTCSegmentSourceAge.WithLabelValues(labels...).Observe(float64(age.Milliseconds()))
	}
	if !timing.IngestReceived.IsZero() && !at.Before(timing.IngestReceived) {
		spmetrics.WebRTCSegmentIngestLatency.WithLabelValues(labels...).
			Observe(float64(at.Sub(timing.IngestReceived).Milliseconds()))
	}
}

const (
	playbackTargetAge = 4 * time.Second
	playbackDropAge   = 8 * time.Second
)

// playbackSourceAge is stricter than SegmentTiming.SourceAge because callers
// need to distinguish a valid source timestamp at the current instant from
// missing timing metadata. Future timestamps are invalid for recovery too.
func playbackSourceAge(timing *bus.SegmentTiming, now time.Time) (time.Duration, bool) {
	if timing == nil || timing.SourceStart.IsZero() || now.Before(timing.SourceStart) {
		return 0, false
	}
	return now.Sub(timing.SourceStart), true
}

func getPlaybackRateForSourceAge(age time.Duration) float64 {
	switch {
	case age <= playbackTargetAge:
		return 1.0
	case age >= playbackDropAge:
		return 1.5
	default:
		// Linear interpolation between (4s, 1.0) and (8s, 1.5). The
		// drop threshold is handled separately so this rate is bounded.
		progress := (float64(age) - float64(playbackTargetAge)) /
			(float64(playbackDropAge) - float64(playbackTargetAge))
		return 1.0 + (0.5 * progress)
	}
}

func playbackRate(sourceAge, queueDuration time.Duration, hasSourceAge bool) float64 {
	if hasSourceAge {
		return getPlaybackRateForSourceAge(sourceAge)
	}
	return getPlaybackRate(queueDuration)
}

func shouldDropStaleGOP(sourceAge time.Duration, hasSourceAge bool) bool {
	return hasSourceAge && sourceAge > playbackDropAge
}

// discardStaleGOPs removes complete packetized segments from the head of the
// playback stream until a segment near the live edge is available. A
// PacketizedSegment is the packetized form of one source GOP, so this keeps
// the drop boundary safe for video reference frames. The final stale segment
// is retained when no replacement is available, so playback always has media
// to send while waiting for the next GOP.
func discardStaleGOPs(current *bus.PacketizedSegment, next func() (*bus.PacketizedSegment, bool), now time.Time) (*bus.PacketizedSegment, int, time.Duration) {
	dropped := 0
	queuedDroppedDuration := time.Duration(0)
	fromQueue := false
	for current != nil {
		age, hasSourceAge := playbackSourceAge(current.Timing, now)
		if !shouldDropStaleGOP(age, hasSourceAge) {
			return current, dropped, queuedDroppedDuration
		}
		nextSegment, ok := next()
		if !ok {
			// Dropping the current segment without a replacement would leave the
			// viewer with no packetized media to play.
			return current, dropped, queuedDroppedDuration
		}
		if fromQueue {
			queuedDroppedDuration += current.Duration
		}
		dropped++
		current = nextSegment
		fromQueue = true
	}
	return nil, dropped, queuedDroppedDuration
}

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCPlayback2(ctx context.Context, user string, rendition string, offer *webrtc.SessionDescription, viewer string) (*webrtc.SessionDescription, error) {
	playbackStarted := time.Now()
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

	closePeer := func() {
		if cErr := peerConnection.Close(); cErr != nil {
			log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
		}
	}

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(*offer); err != nil {
		closePeer()
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		closePeer()
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		closePeer()
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

	playbackCtx, playbackCancel := context.WithCancel(ctx)
	connected := make(chan struct{})
	var connectedOnce sync.Once
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Log(ctx, "Connection State has changed", "state", connectionState.String())
	})
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Log(ctx, "Peer Connection State has changed", "state", s.String())

		if s == webrtc.PeerConnectionStateConnected {
			connectedOnce.Do(func() { close(connected) })
			markConnected()
		}

		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateDisconnected {
			log.Log(ctx, "Peer Connection has gone to failed, exiting")
			playbackCancel()
		}
	})

	go func() {
		ctx := playbackCtx
		defer playbackCancel()
		defer markDone()

		go func() {
			<-ctx.Done()
			if cErr := peerConnection.Close(); cErr != nil {
				log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
			}
		}()

		// Samples written before the sender is connected can be lost. Wait until
		// the peer is ready so a finite cached GOP is still available to play.
		select {
		case <-connected:
		case <-ctx.Done():
			return
		}

		latency := time.Duration(0)
		var latencyMu sync.Mutex
		var firstSend sync.Once

		packetQueue := make(chan *bus.PacketizedSegment, 1024)
		go func() {
			busRendition := rendition
			if audioOnly || rendition == "source" {
				busRendition = WebRTCSourceRendition
			}
			// Replay only the live-edge GOP. Replaying older GOPs adds their media
			// duration to startup and makes first-RTP latency grow with the cache.
			segChan := mm.bus.SubscribeSegmentBuf(ctx, user, busRendition, 1)
			defer mm.bus.UnsubscribeSegment(ctx, user, busRendition, segChan)
			seenSegmentIDs := make(map[string]struct{}, 32)
			seenOrder := make([]string, 0, 32)
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
					if timing := file.PacketizedData.Timing; timing != nil && timing.SegmentID != "" {
						if _, seen := seenSegmentIDs[timing.SegmentID]; seen {
							continue
						}
						seenSegmentIDs[timing.SegmentID] = struct{}{}
						seenOrder = append(seenOrder, timing.SegmentID)
						if len(seenOrder) > 64 {
							delete(seenSegmentIDs, seenOrder[0])
							seenOrder = seenOrder[1:]
						}
					}
					packetCopy := *file.PacketizedData
					packetCopy.Timing = file.PacketizedData.Timing.Clone()
					packet := &packetCopy
					now := time.Now()
					if packet.Timing != nil {
						packet.Timing.WebRTCQueued = now
						if age := packet.Timing.SourceAge(now); age > 0 {
							spmetrics.WebRTCSourceAge.WithLabelValues(packet.Streamer, packet.Rendition, "queued").
								Observe(float64(age.Milliseconds()))
						}
					}
					latencyMu.Lock()
					latency += packet.Duration
					latencyMu.Unlock()
					select {
					case packetQueue <- packet:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case packet := <-packetQueue:
					latencyMu.Lock()
					latency -= packet.Duration
					if latency < 0 {
						latency = 0
					}
					latencyMu.Unlock()

					initialStreamer := packet.Streamer
					initialRendition := packet.Rendition
					now := time.Now()
					if sourceAge, hasSourceAge := playbackSourceAge(packet.Timing, now); shouldDropStaleGOP(sourceAge, hasSourceAge) {
						packet, dropped, queuedDroppedDuration := discardStaleGOPs(packet, func() (*bus.PacketizedSegment, bool) {
							select {
							case next, ok := <-packetQueue:
								if !ok {
									return nil, false
								}
								return next, true
							default:
								return nil, false
							}
						}, now)
						latencyMu.Lock()
						latency -= queuedDroppedDuration
						if latency < 0 {
							latency = 0
						}
						latencyMu.Unlock()
						if dropped > 0 {
							spmetrics.WebRTCStaleGOPDroppedTotal.WithLabelValues(initialStreamer, initialRendition).
								Add(float64(dropped))
						}
						if packet == nil {
							continue
						}
					}

					var segmentFirstSend sync.Once
					latencyMu.Lock()
					currentLatency := latency
					latencyMu.Unlock()
					queuedLatency := currentLatency + packet.Duration
					spmetrics.WebRTCQueueDuration.WithLabelValues(packet.Streamer, packet.Rendition).
						Observe(float64(queuedLatency.Milliseconds()))
					segmentID := ""
					sourceStart := time.Time{}
					packetizeDuration := time.Duration(0)
					packetizeQueueDuration := time.Duration(0)
					sourceAge := time.Duration(0)
					hasSourceAge := false
					if packet.Timing != nil {
						segmentID = packet.Timing.SegmentID
						sourceStart = packet.Timing.SourceStart
						sourceAge, hasSourceAge = playbackSourceAge(packet.Timing, time.Now())
						if !packet.Timing.PacketizeStarted.IsZero() && !packet.Timing.PacketizeCompleted.IsZero() {
							packetizeDuration = packet.Timing.PacketizeCompleted.Sub(packet.Timing.PacketizeStarted)
						}
						if !packet.Timing.Distributed.IsZero() && !packet.Timing.PacketizeStarted.IsZero() {
							packetizeQueueDuration = packet.Timing.PacketizeStarted.Sub(packet.Timing.Distributed)
						}
					}
					scalar := playbackRate(sourceAge, currentLatency, hasSourceAge)
					spmetrics.WebRTCLocalQueueDuration.WithLabelValues(packet.Streamer, packet.Rendition).
						Set(float64(currentLatency.Milliseconds()))
					spmetrics.WebRTCOldestSourceAge.WithLabelValues(packet.Streamer, packet.Rendition).
						Set(float64(sourceAge.Milliseconds()))
					log.Debug(ctx, "playback latency",
						"segment_id", segmentID,
						"streamer", packet.Streamer,
						"rendition", packet.Rendition,
						"latency", currentLatency,
						"scalar", scalar,
						"source_start", sourceStart,
						"source_age_ms", sourceAge.Milliseconds(),
						"packetize_ms", packetizeDuration.Milliseconds(),
						"packetize_queue_ms", packetizeQueueDuration.Milliseconds(),
						"webrtc_queue_ms", queuedLatency.Milliseconds(),
						"webrtc_source_age_ms", sourceAge.Milliseconds())
					g, _ := errgroup.WithContext(ctx)
					wroteAny := false
					recordFirstSend := func() {
						segmentFirstSend.Do(func() {
							firstSendAt := time.Now()
							if packet.Timing != nil {
								packet.Timing.WebRTCSegmentFirstSent = firstSendAt
							}
							observeWebRTCSegmentLatency(packet.Timing, packet.Streamer, packet.Rendition, firstSendAt, "first_rtp")
						})
						firstSend.Do(func() {
							firstSendAt := time.Now()
							if packet.Timing != nil {
								packet.Timing.WebRTCFirstSent = firstSendAt
								if age := packet.Timing.SourceAge(firstSendAt); age > 0 {
									spmetrics.WebRTCSourceAge.WithLabelValues(packet.Streamer, packet.Rendition, "first_sent").
										Observe(float64(age.Milliseconds()))
								}
							}
							spmetrics.WebRTCSetupToFirstSendDuration.WithLabelValues(packet.Streamer, packet.Rendition).
								Observe(float64(firstSendAt.Sub(playbackStarted).Milliseconds()))
						})
					}

					if !audioOnly && len(packet.Video) > 0 {
						wroteAny = true
						g.Go(func() error {
							started := time.Now()
							err := writeSamples(ctx, videoTrack, packet.Video, scalar, recordFirstSend)
							spmetrics.WebRTCVideoSendDuration.WithLabelValues(packet.Streamer, packet.Rendition).
								Observe(float64(time.Since(started).Milliseconds()))
							return err
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
					var audioDur time.Duration
					if len(packet.Audio) > 0 {
						audioDur = packet.Duration / time.Duration(len(packet.Audio))
					}
					if audioDur > 0 {
						wroteAny = true
						g.Go(func() error {
							started := time.Now()
							ticker := time.NewTicker(time.Duration(float64(audioDur) * (1 / scalar)))
							defer ticker.Stop()
							for i, audio := range packet.Audio {
								err := audioTrack.WriteSample(media.Sample{Data: audio.Data, Duration: audioDur})
								if err != nil {
									return fmt.Errorf("failed to write audio sample: %w", err)
								}
								if i == 0 {
									recordFirstSend()
								}
								select {
								case <-ctx.Done():
									return nil
								case <-ticker.C:
									continue
								}
							}
							spmetrics.WebRTCAudioSendDuration.WithLabelValues(packet.Streamer, packet.Rendition).
								Observe(float64(time.Since(started).Milliseconds()))
							return nil
						})
					} else {
						log.Warn(ctx, "no audio samples to write")
					}
					if wroteAny {
						if err := g.Wait(); err != nil {
							log.Error(ctx, "failed to write samples", "error", err)
							playbackCancel()
						} else {
							completedAt := time.Now()
							if packet.Timing != nil {
								packet.Timing.WebRTCSegmentCompleted = completedAt
							}
							observeWebRTCSegmentLatency(packet.Timing, packet.Streamer, packet.Rendition, completedAt, "complete")
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

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := audioRTPSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

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
func writeSamples(ctx context.Context, track *webrtc.TrackLocalStaticSample, samples []bus.PacketizedSample, scalar float64, onFirstSample func()) error {
	start := time.Now()
	var scheduled time.Duration
	for i, s := range samples {
		if err := track.WriteSample(media.Sample{Data: s.Data, Duration: s.Duration}); err != nil {
			return fmt.Errorf("failed to write sample: %w", err)
		}
		if i == 0 {
			onFirstSample()
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

// getPlaybackRate is the compatibility fallback for packets without source
// timing. Timed packets use getPlaybackRateForSourceAge instead.
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
