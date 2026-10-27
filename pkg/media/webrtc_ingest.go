package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/rtcrec"
)

// WebRTCIngest is the in-process WHIP entry: it builds the signing element via
// SegmentAndSignElem (→ ValidateMP4) and runs the shared ingest pipeline. Stays
// in scope for the duration of a single stream.
func (mm *MediaManager) WebRTCIngest(ctx context.Context, offer *webrtc.SessionDescription, signer MediaSigner, peerConnection rtcrec.PeerConnection, done chan error) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String(), "mediafunc", "WebRTCIngest", "streamer", signer.Streamer())
	ctx, cancel := context.WithCancel(ctx)
	signerElem, err := mm.SegmentAndSignElem(ctx, signer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed create signer element: %w", err)
	}
	return mm.webRTCIngestPipeline(ctx, cancel, offer, peerConnection, signerElem, signer, done)
}

// webRTCIngestPipeline runs WebRTC ingest over a pre-built signer element:
// depay/parse the incoming RTP into the muxl signing bin, answer the offer, and
// stream in the background. The in-process path passes a SegmentAndSignElem (→
// ValidateMP4) and the streamer's signer (for key revocation); the isolated WHIP
// worker passes a muxlSignSegmentElem wired to its frame socket and a nil
// keyRevSigner. The cancellable ctx and signerElem are built by the caller (the
// signer element's goroutines are tied to ctx).
func (mm *MediaManager) webRTCIngestPipeline(ctx context.Context, cancel context.CancelFunc, offer *webrtc.SessionDescription, peerConnection rtcrec.PeerConnection, signerElem gst.Element, keyRevSigner MediaSigner, done chan error) (*webrtc.SessionDescription, error) {
	// Allow us to receive 1 audio track, and 1 video track
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
	} else if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to add video transceiver: %w", err)
	}

	// The peer connection takes this one video codec (see NewPeerConnection).
	videoCodec := ingestVideoCodec(offer)
	videoBranch := "appsrc format=time is-live=true do-timestamp=true name=videosrc ! capsfilter caps=application/x-rtp ! rtph264depay ! capsfilter caps=video/x-h264,stream-format=byte-stream,alignment=nal ! h264parse name=videoparse disable-passthrough=true config-interval=-1 ! h264timestamper ! identity ! queue.sink_0"
	if videoCodec.Name == VideoAV1.Name {
		// The build has no rtpav1depay: OnTrack depacketizes AV1 in Go (see
		// pushAV1) and pushes whole temporal units, stamped from their RTP
		// timestamps.
		videoBranch = fmt.Sprintf("appsrc format=time is-live=true name=videosrc caps=\"%s\" ! %s ! queue.sink_0", VideoAV1.SampleCaps, VideoAV1.ParserLaunch("videoparse", ""))
	}
	pipelineSlice := []string{
		"multiqueue name=queue",
		videoBranch,
		"appsrc format=time do-timestamp=true name=audiosrc ! capsfilter caps=application/x-rtp,media=audio,encoding-name=OPUS,payload=111 ! rtpopusdepay ! opusparse ! queue.sink_1",
	}

	pipelineElem, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: not a pipeline")
	}

	// Only a frame a decoder can start on starts a segment (see installKeyframeProbe).
	if parseEle := pipeline.GetByName("videoparse"); parseEle != nil {
		streamer := "whip"
		if keyRevSigner != nil {
			streamer = keyRevSigner.Streamer()
		}
		installKeyframeProbe(ctx, videoCodec, parseEle.GetStaticPad("src"), streamer)
	}
	queue := pipeline.GetByName("queue")
	if queue == nil {
		return nil, fmt.Errorf("failed to get queue element from pipeline")
	}

	// multiqueue pairs sink_N with src_N: video is sink_0, audio is sink_1.
	videoSrcPad := queue.GetStaticPad("src_0")
	audioSrcPad := queue.GetStaticPad("src_1")
	if videoSrcPad == nil || audioSrcPad == nil {
		return nil, fmt.Errorf("failed to get videoSrcPads from queue")
	}

	videoSrcElem := pipeline.GetByName("videosrc")
	if videoSrcElem == nil {
		return nil, fmt.Errorf("failed to get videoSrcElem element from pipeline")
	}
	videoSrc := videoSrcElem.(app.AppSrc)

	audioSrcElem := pipeline.GetByName("audiosrc")
	if audioSrcElem == nil {
		return nil, fmt.Errorf("failed to get audioSrcElem element from pipeline")
	}
	audioSrc := audioSrcElem.(app.AppSrc)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(*offer); err != nil {
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := rtcrec.GatheringCompletePromise(peerConnection)

	// cancel + signerElem are provided by the caller.
	if !pipeline.Add(signerElem) {
		cancel()
		return nil, fmt.Errorf("failed to add signer element to pipeline")
	}
	// The signer bin exposes its sink pads as video_0 and audio_0.
	signerElemVideoPad := signerElem.GetStaticPad("video_0")
	signerElemAudioPad := signerElem.GetStaticPad("audio_0")
	if signerElemVideoPad == nil || signerElemAudioPad == nil {
		cancel()
		return nil, fmt.Errorf("failed to get signerElemPads from signer element")
	}
	linked := videoSrcPad.Link(signerElemVideoPad)
	if linked != gst.PadLinkOK {
		cancel()
		return nil, fmt.Errorf("failed to link videoSrcPad to signerElemVideoPad")
	}
	linked = audioSrcPad.Link(signerElemAudioPad)
	if linked != gst.PadLinkOK {
		cancel()
		return nil, fmt.Errorf("failed to link audioSrcPad to signerElemAudioPad")
	}

	// Setup complete! Now we boot up streaming in the background while returning the SDP offer to the user.
	go func() {
		busErrorChan := make(chan error)
		go func() {
			err := HandleBusMessages(ctx, pipeline)
			if err != nil {
				log.Log(ctx, "pipeline error", "error", err)
			}
			cancel()
			busErrorChan <- err
		}()

		defer cancel()
		defer func() { done <- <-busErrorChan }()

		go func() {
			ticker := time.NewTicker(time.Second * 1)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					state := pipeline.GetCurrentState()
					log.Debug(ctx, "pipeline state", "state", state)
				}
			}
		}()

		// subscription to bus messages for key revocation (in-process only; the
		// isolated worker has no model-backed signer to revoke against)
		if keyRevSigner != nil {
			go mm.HandleKeyRevocation(ctx, keyRevSigner, pipeline)
		}

		go func() {
			<-ctx.Done()
			if cErr := peerConnection.Close(); cErr != nil {
				log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
			}
		}()

		log.Debug(ctx, "starting pipeline")

		// Start the pipeline
		if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
			log.Log(ctx, "failed to set pipeline state")
			cancel()
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

			if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateDisconnected {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has ended, exiting", "state", s.String())
				cancel()
			}
		})

		videoFirst := false
		audioFirst := false

		peerConnection.OnTrack(func(track rtcrec.TrackRemote, _ rtcrec.RTPReceiver) {
			log.Warn(ctx, "OnTrack", "kind", track.Kind())
			if track.Kind() == webrtc.RTPCodecTypeVideo {
				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(time.Second * 1)
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
							if rtcpSendErr != nil {
								log.Log(ctx, "failed to send rtcp packet", "error", rtcpSendErr)
								cancel()
								return
							}
						}
					}
				}()

				codecName := strings.Split(track.Codec().MimeType, "/")[1]
				log.Log(ctx, "Track has started", "payloadType", track.PayloadType(), "codecName", codecName)

				if strings.EqualFold(track.Codec().MimeType, VideoAV1.MimeType) {
					pushAV1(ctx, cancel, track, videoSrc)
					return
				}
				buf := make([]byte, 1400)
				for {
					i, _, readErr := track.Read(buf)
					if readErr != nil {
						log.Log(ctx, "failed to read track", "error", readErr)
						videoSrc.EndOfStream()
						return
					}
					// if ctx.Err() != nil {
					// 	return
					// }
					if !videoFirst {
						videoFirst = true
						log.Debug(ctx, "got video data", "len", len(buf[:i]))
					}

					gbuf, bufErr := newBufferFromBytes(buf[:i])
					if bufErr != nil {
						log.Log(ctx, "failed to build buffer", "error", bufErr)
						cancel()
						return
					}

					ret := videoSrc.PushBuffer(gbuf)
					if ret != gst.FlowOK {
						log.Log(ctx, "failed to push buffer", "error", ret)
						cancel()
						return
					}
					// state := pipeline.GetCurrentState()
					// if state != gst.StatePlaying {
					// 	log.Warn(ctx, "pipeline state is not playing, consider running with GST_DEBUG=*:5 to find out why", "state", state)
					// 	cancel()
					// 	return
					// }
				}
			}
			if track.Kind() == webrtc.RTPCodecTypeAudio {

				codecName := strings.Split(track.Codec().MimeType, "/")[1]
				log.Log(ctx, "Track has started", "payloadType", track.PayloadType(), "codecName", codecName)

				buf := make([]byte, 1400)
				for {
					i, _, readErr := track.Read(buf)
					if readErr != nil {
						log.Log(ctx, "failed to read track", "error", readErr)
						audioSrc.EndOfStream()
						return
					}
					// if ctx.Err() != nil {
					// 	return
					// }
					if !audioFirst {
						audioFirst = true
						log.Debug(ctx, "got audio data", "len", len(buf[:i]))
					}

					gbuf, bufErr := newBufferFromBytes(buf[:i])
					if bufErr != nil {
						log.Log(ctx, "failed to build buffer", "error", bufErr)
						cancel()
						return
					}
					ret := audioSrc.PushBuffer(gbuf)
					if ret != gst.FlowOK {
						log.Log(ctx, "failed to push buffer", "error", ret)
						cancel()
						return
					}
					// state := pipeline.GetCurrentState()
					// if state != gst.StatePlaying {
					// 	log.Warn(ctx, "pipeline state is not playing, consider running with GST_DEBUG=*:5 to find out why", "state", state)
					// 	cancel()
					// 	return
					// }
				}
			}
		})

		<-ctx.Done()

		if ret := pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone); ret == gst.StateChangeFailure {
			log.Log(ctx, "failed to set pipeline state to null")
		}

		if ret := audioSrcElem.SetState(gst.StateNull); ret == gst.StateChangeFailure {
			log.Log(ctx, "failed to set audioSrcElem state to null")
		}

		if ret := videoSrcElem.SetState(gst.StateNull); ret == gst.StateChangeFailure {
			log.Log(ctx, "failed to set videoSrcElem state to null")
		}

		log.Log(ctx, "webrtc ingest pipeline done")

	}()
	select {
	case <-gatherComplete:
		return peerConnection.LocalDescription(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// av1MaxLatePackets is how many packets the AV1 sample builder holds while a
// temporal unit is incomplete: more than a 4K key frame spans, which would
// otherwise lose its head before its tail arrives.
const av1MaxLatePackets = 1 << 14

// pushAV1 depacketizes an AV1 track in Go, as the build has no rtpav1depay:
// a sample builder reorders the RTP and assembles temporal units, which go to
// src stamped from their RTP timestamps, starting at src's running time at
// the first one (the clock the audio's do-timestamp stamps by). Nothing goes
// before the first sequence header, whose av1C src's caps then carry as
// codec_data (see AV1CodecConfig): no decoder could start sooner anyway.
func pushAV1(ctx context.Context, cancel context.CancelFunc, track rtcrec.TrackRemote, src app.AppSrc) {
	sb := samplebuilder.New(av1MaxLatePackets, &codecs.AV1Depacketizer{}, 90000, samplebuilder.WithMaxTimeDelay(time.Second/2))
	var base time.Duration
	var ticks int64 // 90 kHz, since the first temporal unit
	var last uint32
	started := false
	buf := make([]byte, 1500)
	for {
		n, _, err := track.Read(buf)
		if err != nil {
			log.Log(ctx, "failed to read track", "error", err)
			src.EndOfStream()
			return
		}
		pkt := &rtp.Packet{}
		// The sample builder keeps the packet, so it gets bytes of its own.
		if err := pkt.Unmarshal(bytes.Clone(buf[:n])); err != nil {
			log.Debug(ctx, "dropping malformed RTP packet", "error", err)
			continue
		}
		sb.Push(pkt)
		for s := sb.Pop(); s != nil; s = sb.Pop() {
			if len(s.Data) == 0 {
				continue
			}
			if !started {
				seq := tuSequenceHeader(s.Data)
				if seq == nil {
					continue
				}
				caps, err := av1CodecDataCaps(seq)
				if err != nil {
					log.Log(ctx, "dropping AV1 temporal unit", "error", err)
					continue
				}
				src.AppSrcSetCaps(gst.CapsFromString(caps))
				started = true
				base, _ = clockTimeDuration(src.GetCurrentRunningTime())
			} else {
				ticks += int64(int32(s.PacketTimestamp - last))
			}
			last = s.PacketTimestamp
			gbuf, err := newBufferFromBytes(s.Data)
			if err != nil {
				log.Log(ctx, "failed to build buffer", "error", err)
				cancel()
				return
			}
			pts := gst.ClockTime(base + time.Duration(ticks/90000)*time.Second + time.Duration(ticks%90000)*time.Second/90000)
			gbuf.SetPTS(pts)
			gbuf.SetDTS(pts)
			if ret := src.PushBuffer(gbuf); ret != gst.FlowOK {
				log.Log(ctx, "failed to push buffer", "error", ret)
				cancel()
				return
			}
		}
	}
}
