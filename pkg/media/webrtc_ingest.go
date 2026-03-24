package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/rtcrec"
)

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCIngest(ctx context.Context, offer *webrtc.SessionDescription, signer MediaSigner, peerConnection rtcrec.PeerConnection, done chan error) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	ctx = log.WithLogValues(ctx, "webrtcID", uu.String(), "mediafunc", "WebRTCIngest", "streamer", signer.Streamer())

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to add video transceiver: %w", err)
	}

	pipelineSlice := []string{
		"multiqueue name=queue",
		"appsrc format=time is-live=true do-timestamp=true name=videosrc ! capsfilter caps=application/x-rtp ! rtph264depay ! capsfilter caps=video/x-h264,stream-format=byte-stream,alignment=nal ! h264parse disable-passthrough=true config-interval=-1 ! h264timestamper ! identity ! queue.sink_0",
		"appsrc format=time do-timestamp=true name=audiosrc ! capsfilter caps=application/x-rtp,media=audio,encoding-name=OPUS,payload=111 ! rtpopusdepay ! opusparse ! queue.sink_1",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	queue, err := pipeline.GetElementByName("queue")
	if err != nil {
		return nil, fmt.Errorf("failed to get queue element from pipeline: %w", err)
	}

	// err = queue.Link(signerElem)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to link queue to signer element: %w", err)
	// }
	videoSrcPads, err := queue.GetSrcPads()
	if err != nil {
		return nil, fmt.Errorf("failed to get videoSrcPads from queue: %w", err)
	}
	if len(videoSrcPads) != 2 {
		return nil, fmt.Errorf("failed to get videoSrcPads from queue")
	}
	videoSrcPad := videoSrcPads[0]
	audioSrcPad := videoSrcPads[1]

	videoSrcElem, err := pipeline.GetElementByName("videosrc")
	if err != nil {
		return nil, fmt.Errorf("failed to get videoSrcElem element from pipeline: %w", err)
	}
	videoSrc := app.SrcFromElement(videoSrcElem)

	audioSrcElem, err := pipeline.GetElementByName("audiosrc")
	if err != nil {
		return nil, fmt.Errorf("failed to get audioSrcElem element from pipeline: %w", err)
	}
	audioSrc := app.SrcFromElement(audioSrcElem)

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

	ctx, cancel := context.WithCancel(ctx)
	signerElem, err := mm.SegmentAndSignElem(ctx, signer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed create signer element: %w", err)
	}
	err = pipeline.Add(signerElem)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to add signer element to pipeline: %w", err)
	}
	signerElemPads, err := signerElem.GetPads()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get signerElemPads from signer element: %w", err)
	}
	if len(signerElemPads) != 2 {
		cancel()
		return nil, fmt.Errorf("failed to get signerElemPads from signer element")
	}
	signerElemVideoPad := signerElemPads[0]
	signerElemAudioPad := signerElemPads[1]
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

		// subscription to bus messages for key revocation
		go mm.HandleKeyRevocation(ctx, signer, pipeline)

		go func() {
			<-ctx.Done()
			if cErr := peerConnection.Close(); cErr != nil {
				log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
			}
		}()

		log.Debug(ctx, "starting pipeline")

		// Start the pipeline
		err = pipeline.SetState(gst.StatePlaying)
		if err != nil {
			log.Log(ctx, "failed to set pipeline state", "error", err)
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

				// appSrc := pipelineForCodec(track, codecName)
				buf := make([]byte, 1400)
				for {
					i, _, readErr := track.Read(buf)
					if readErr != nil {
						log.Log(ctx, "failed to read track", "error", readErr)
						videoSrc.EndStream()
						return
					}
					// if ctx.Err() != nil {
					// 	return
					// }
					if !videoFirst {
						videoFirst = true
						log.Debug(ctx, "got video data", "len", len(buf[:i]))
					}

					gbuf := gst.NewBufferWithSize(int64(len(buf[:i])))
					gbuf.Map(gst.MapWrite).WriteData(buf[:i])
					gbuf.Unmap()

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
						audioSrc.EndStream()
						return
					}
					// if ctx.Err() != nil {
					// 	return
					// }
					if !audioFirst {
						audioFirst = true
						log.Debug(ctx, "got audio data", "len", len(buf[:i]))
					}

					gbuf := gst.NewBufferWithSize(int64(len(buf[:i])))
					gbuf.Map(gst.MapWrite).WriteData(buf[:i])
					gbuf.Unmap()
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

		if err := pipeline.BlockSetState(gst.StateNull); err != nil {
			log.Log(ctx, "failed to set pipeline state to null", "error", err)
		}

		if err := audioSrcElem.SetState(gst.StateNull); err != nil {
			log.Log(ctx, "failed to set audioSrcElem state to null", "error", err)
		}

		if err := videoSrcElem.SetState(gst.StateNull); err != nil {
			log.Log(ctx, "failed to set videoSrcElem state to null", "error", err)
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
