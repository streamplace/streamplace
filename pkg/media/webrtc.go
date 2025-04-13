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
	"github.com/pion/webrtc/v4/pkg/media"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// we have a bug that prevents us from correctly probing video durations
// a lot of the time. so when we don't have them we use the last duration
// that we had, and when we don't have that we use a default duration
var DEFAULT_DURATION = time.Duration(32 * time.Millisecond)

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCPlayback(ctx context.Context, user string, rendition string, offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "mediafunc", "WebRTCPlayback")

	pipelineSlice := []string{
		"h264parse name=videoparse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"opusparse name=audioparse ! appsink name=audioappsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	outputQueue, done, err := ConcatStream(ctx, pipeline, user, rendition, mm)
	if err != nil {
		return nil, fmt.Errorf("failed to get output queue: %w", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()
	// queuePadVideo := outputQueue.GetRequestPad("src_%u")
	// if queuePadVideo == nil {
	// 	return nil, fmt.Errorf("failed to get queue video pad")
	// }
	// queuePadAudio := outputQueue.GetRequestPad("src_%u")
	// if queuePadAudio == nil {
	// 	return nil, fmt.Errorf("failed to get queue audio pad")
	// }

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return nil, fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	err = outputQueue.Link(videoParse)
	if err != nil {
		return nil, fmt.Errorf("failed to link output queue to video parse: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	err = outputQueue.Link(audioParse)
	if err != nil {
		return nil, fmt.Errorf("failed to link output queue to audio parse: %w", err)
	}

	videoappsinkele, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}

	audioappsinkele, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio sink element from pipeline: %w", err)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC peer connection: %w", err)
	}
	go func() {
		<-ctx.Done()
		if cErr := peerConnection.Close(); cErr != nil {
			log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
		}
	}()

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}
	videoRTPSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add video track to peer connection: %w", err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}
	audioRTPSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to add audio track to peer connection: %w", err)
	}

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
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Setup complete! Now we boot up streaming in the background while returning the SDP offer to the user.

	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
	}()

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

	var lastVideoDuration = &DEFAULT_DURATION

	go func() {

		videoappsink := app.SinkFromElement(videoappsinkele)
		videoappsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
				sample := sink.PullSample()
				if sample == nil {
					return gst.FlowEOS
				}

				buffer := sample.GetBuffer()
				if buffer == nil {
					return gst.FlowError
				}

				samples := buffer.Map(gst.MapRead).Bytes()
				defer buffer.Unmap()
				clockTime := buffer.Duration()
				dur := clockTime.AsDuration()
				mediaSample := media.Sample{Data: samples}
				if dur != nil {
					mediaSample.Duration = *dur
					lastVideoDuration = dur
				} else if lastVideoDuration != nil {
					mediaSample.Duration = *lastVideoDuration
				} else {
					log.Log(ctx, "no video duration", "samples", len(samples))
					// cancel()
					return gst.FlowOK
				}

				if err := videoTrack.WriteSample(mediaSample); err != nil {
					log.Log(ctx, "failed to write video sample", "error", err)
					cancel()
				}

				return gst.FlowOK
			},
			EOSFunc: func(sink *app.Sink) {
				log.Warn(ctx, "videoappsink EOSFunc")
				cancel()
			},
		})

		audioappsink := app.SinkFromElement(audioappsinkele)
		audioappsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
				sample := sink.PullSample()
				if sample == nil {
					return gst.FlowEOS
				}

				buffer := sample.GetBuffer()
				if buffer == nil {
					return gst.FlowError
				}

				samples := buffer.Map(gst.MapRead).Bytes()
				defer buffer.Unmap()

				clockTime := buffer.Duration()
				dur := clockTime.AsDuration()
				mediaSample := media.Sample{Data: samples}
				if dur != nil {
					mediaSample.Duration = *dur
				} else {
					log.Log(ctx, "no audio duration", "samples", len(samples))
					// cancel()
					return gst.FlowOK
				}
				if err := audioTrack.WriteSample(mediaSample); err != nil {
					log.Log(ctx, "failed to write audio sample", "error", err)
					return gst.FlowOK
				}

				return gst.FlowOK
			},
			EOSFunc: func(sink *app.Sink) {
				log.Warn(ctx, "audioappsink EOSFunc")
				cancel()
			},
		})

		// Start the pipeline
		pipeline.SetState(gst.StatePlaying)
		spmetrics.ViewerInc(user)
		defer spmetrics.ViewerDec(user)

		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := videoRTPSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

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
			if connectionState == webrtc.ICEConnectionStateConnected {
				// iceConnectedCtxCancel()
			}
		})

		// Set the handler for Peer connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
			log.Log(ctx, "Peer Connection State has changed", "state", s.String())

			if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has gone to failed, exiting")
				cancel()
			}
		})

		<-ctx.Done()
	}()
	select {
	case <-gatherComplete:
		return peerConnection.LocalDescription(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCIngest(ctx context.Context, offer *webrtc.SessionDescription, signer MediaSigner) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String(), "mediafunc", "WebRTCIngest")

	// Setup the codecs you want to use.
	// We'll use a VP8 and Opus but you can also define your own

	// Create a new RTCPeerConnection
	peerConnection, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC peer connection: %w", err)
	}

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to add video transceiver: %w", err)
	}

	pipelineSlice := []string{
		"multiqueue name=queue",
		"appsrc format=time is-live=true do-timestamp=true name=videosrc ! capsfilter caps=application/x-rtp ! rtph264depay ! capsfilter caps=video/x-h264,stream-format=byte-stream,alignment=nal ! h264parse ! h264timestamper ! identity ! queue.sink_0",
		"appsrc format=time is-live=true do-timestamp=true name=audiosrc ! capsfilter caps=application/x-rtp,media=audio,encoding-name=OPUS,payload=111 ! rtpopusdepay ! opusdec use-inband-fec=true ! audiorate ! opusenc ! queue.sink_1",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	queue, err := pipeline.GetElementByName("queue")
	if err != nil {
		return nil, fmt.Errorf("failed to get queue element from pipeline: %w", err)
	}

	signerElem, err := mm.SegmentAndSignElem(ctx, signer)
	if err != nil {
		return nil, fmt.Errorf("failed create signer element: %w", err)
	}
	err = pipeline.Add(signerElem)
	if err != nil {
		return nil, fmt.Errorf("failed to add signer element to pipeline: %w", err)
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

	signerElemPads, err := signerElem.GetPads()
	if err != nil {
		return nil, fmt.Errorf("failed to get signerElemPads from signer element: %w", err)
	}
	if len(signerElemPads) != 2 {
		return nil, fmt.Errorf("failed to get signerElemPads from signer element")
	}
	signerElemVideoPad := signerElemPads[0]
	signerElemAudioPad := signerElemPads[1]
	videoSrcPad.Link(signerElemVideoPad)
	audioSrcPad.Link(signerElemAudioPad)

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

	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
	}()

	go func() {
		<-ctx.Done()
		if cErr := peerConnection.Close(); cErr != nil {
			log.Log(ctx, "cannot close peerConnection: %v\n", cErr)
		}
	}()

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
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

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
	// Setup complete! Now we boot up streaming in the background while returning the SDP offer to the user.

	go func() {
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
			if connectionState == webrtc.ICEConnectionStateConnected {
				// iceConnectedCtxCancel()
			}
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

		log.Warn(ctx, "setting OnTrack")
		peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			log.Warn(ctx, "OnTrack")
			if track.Kind() == webrtc.RTPCodecTypeVideo {
				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(time.Second * 5)
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

				codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
				log.Log(ctx, "Track has started", "payloadType", track.PayloadType(), "codecName", codecName)

				// appSrc := pipelineForCodec(track, codecName)
				buf := make([]byte, 1400)
				for {
					i, _, readErr := track.Read(buf)
					if readErr != nil {
						log.Log(ctx, "failed to read track", "error", readErr)
						cancel()
						return
					}
					log.Debug(ctx, "read video track", "bytes", i)

					ret := videoSrc.PushBuffer(gst.NewBufferFromBytes(buf[:i]))
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

				codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
				log.Log(ctx, "Track has started", "payloadType", track.PayloadType(), "codecName", codecName)

				buf := make([]byte, 1400)
				for {
					i, _, readErr := track.Read(buf)
					if readErr != nil {
						log.Log(ctx, "failed to read track", "error", readErr)
						cancel()
						return
					}
					// log.Log(ctx, "read audio track", "bytes", i)

					ret := audioSrc.PushBuffer(gst.NewBufferFromBytes(buf[:i]))
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
	}()
	select {
	case <-gatherComplete:
		return peerConnection.LocalDescription(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
