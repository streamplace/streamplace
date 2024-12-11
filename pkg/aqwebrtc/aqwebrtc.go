package aqwebrtc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/log"
	aqmedia "aquareum.tv/aquareum/pkg/media"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// This function remains in scope for the duration of a single users' playback
func WebRTCPlayback(ctx context.Context, input io.Reader, offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ToWHEP")

	pipelineSlice := []string{
		"appsrc name=appsrc ! matroskademux name=demux",
		"multiqueue name=queue",
		"demux.video_0 ! queue.sink_0",
		"demux.audio_0 ! queue.sink_1",
		"multiqueue name=outqueue",
		"queue.src_0 ! h264parse name=videoparse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"queue.src_1 ! fdkaacdec ! audioresample ! opusenc inband-fec=true perfect-timestamp=true bitrate=128000 ! appsink name=audioappsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return nil, fmt.Errorf("failed to get appsrc element from pipeline: %w", err)
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: aqmedia.ReaderNeedData(ctx, input),
	})

	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
	}()

	videoappsinkele, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}

	audioappsinkele, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio sink element from pipeline: %w", err)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				// URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
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
		pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
			switch msg.Type() {

			case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
				log.Log(ctx, "got gst.MessageEOS, exiting")
				cancel()
			case gst.MessageError: // Error messages are always fatal
				err := msg.ParseError()
				log.Error(ctx, "gstreamer error", "error", err.Error())
				if debug := err.DebugString(); debug != "" {
					log.Log(ctx, "gstreamer debug", "message", debug)
				}
				cancel()
			default:
				log.Debug(ctx, msg.String())
			}
			return true
		})

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

				if err := videoTrack.WriteSample(media.Sample{Data: samples, Duration: *buffer.Duration().AsDuration()}); err != nil {
					log.Log(ctx, "failed to write video sample", "error", err)
					cancel()
				}

				return gst.FlowOK
			},
			EOSFunc: func(sink *app.Sink) {
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

				if err := audioTrack.WriteSample(media.Sample{Data: samples, Duration: *buffer.Duration().AsDuration()}); err != nil {
					log.Log(ctx, "failed to write audio sample", "error", err)
					cancel()
				}

				return gst.FlowOK
			},
			EOSFunc: func(sink *app.Sink) {
				cancel()
			},
		})

		// Start the pipeline
		pipeline.SetState(gst.StatePlaying)

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

			if s == webrtc.PeerConnectionStateFailed {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has gone to failed exiting")
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
func WebRTCIngest(ctx context.Context, offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "GStreamerFunc", "WebRTCIngest")

	m := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// We'll use a VP8 and Opus but you can also define your own
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	i := &interceptor.Registry{}

	// Register a intervalpli factory
	// This interceptor sends a PLI every 3 seconds. A PLI causes a video keyframe to be generated by the sender.
	// This makes our video seekable and more error resilent, but at a cost of lower picture quality and higher bitrates
	// A real world application should process incoming RTCP packets from viewers and forward them to senders
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, fmt.Errorf("failed to create intervalpli factory: %w", err)
	}
	i.Add(intervalPliFactory)

	// Use the default set of Interceptors
	if err = webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, fmt.Errorf("failed to register default interceptors: %w", err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC peer connection: %w", err)
	}

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("failed to add video transceiver: %w", err)
	}

	// pipelineSlice := []string{
	// 	"appsrc name=appsrc ! matroskademux name=demux",
	// 	"multiqueue name=queue",
	// 	"demux.video_0 ! queue.sink_0",
	// 	"demux.audio_0 ! queue.sink_1",
	// 	"multiqueue name=outqueue",
	// 	"queue.src_0 ! h264parse name=videoparse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
	// 	"queue.src_1 ! fdkaacdec ! audioresample ! opusenc inband-fec=true perfect-timestamp=true bitrate=128000 ! appsink name=audioappsink",
	// }

	// pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	// }

	// appsrc, err := pipeline.GetElementByName("appsrc")
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get appsrc element from pipeline: %w", err)
	// }

	// src := app.SrcFromElement(appsrc)
	// src.SetCallbacks(&app.SourceCallbacks{
	// 	NeedDataFunc: aqmedia.ReaderNeedData(ctx, input),
	// })

	// go func() {
	// 	<-ctx.Done()
	// 	pipeline.BlockSetState(gst.StateNull)
	// }()

	// videoappsinkele, err := pipeline.GetElementByName("videoappsink")
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	// }

	// audioappsinkele, err := pipeline.GetElementByName("audioappsink")
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get audio sink element from pipeline: %w", err)
	// }

	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create WebRTC peer connection: %w", err)
	// }
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

	// Setup complete! Now we boot up streaming in the background while returning the SDP offer to the user.

	go func() {
		// pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		// 	switch msg.Type() {

		// 	case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
		// 		log.Log(ctx, "got gst.MessageEOS, exiting")
		// 		cancel()
		// 	case gst.MessageError: // Error messages are always fatal
		// 		err := msg.ParseError()
		// 		log.Error(ctx, "gstreamer error", "error", err.Error())
		// 		if debug := err.DebugString(); debug != "" {
		// 			log.Log(ctx, "gstreamer debug", "message", debug)
		// 		}
		// 		cancel()
		// 	default:
		// 		log.Debug(ctx, msg.String())
		// 	}
		// 	return true
		// })

		// videoappsink := app.SinkFromElement(videoappsinkele)
		// videoappsink.SetCallbacks(&app.SinkCallbacks{
		// 	NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
		// 		sample := sink.PullSample()
		// 		if sample == nil {
		// 			return gst.FlowEOS
		// 		}

		// 		buffer := sample.GetBuffer()
		// 		if buffer == nil {
		// 			return gst.FlowError
		// 		}

		// 		samples := buffer.Map(gst.MapRead).Bytes()
		// 		defer buffer.Unmap()

		// 		if err := videoTrack.WriteSample(media.Sample{Data: samples, Duration: *buffer.Duration().AsDuration()}); err != nil {
		// 			log.Log(ctx, "failed to write video sample", "error", err)
		// 			cancel()
		// 		}

		// 		return gst.FlowOK
		// 	},
		// 	EOSFunc: func(sink *app.Sink) {
		// 		cancel()
		// 	},
		// })

		// audioappsink := app.SinkFromElement(audioappsinkele)
		// audioappsink.SetCallbacks(&app.SinkCallbacks{
		// 	NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
		// 		sample := sink.PullSample()
		// 		if sample == nil {
		// 			return gst.FlowEOS
		// 		}

		// 		buffer := sample.GetBuffer()
		// 		if buffer == nil {
		// 			return gst.FlowError
		// 		}

		// 		samples := buffer.Map(gst.MapRead).Bytes()
		// 		defer buffer.Unmap()

		// 		if err := audioTrack.WriteSample(media.Sample{Data: samples, Duration: *buffer.Duration().AsDuration()}); err != nil {
		// 			log.Log(ctx, "failed to write audio sample", "error", err)
		// 			cancel()
		// 		}

		// 		return gst.FlowOK
		// 	},
		// 	EOSFunc: func(sink *app.Sink) {
		// 		cancel()
		// 	},
		// })

		// Start the pipeline
		// pipeline.SetState(gst.StatePlaying)

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

			if s == webrtc.PeerConnectionStateFailed {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has gone to failed exiting")
				cancel()
			}
		})

		peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			if track.Kind() == webrtc.RTPCodecTypeVideo {
				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(time.Second * 3)
					for range ticker.C {
						rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
						if rtcpSendErr != nil {
							fmt.Println(rtcpSendErr)
						}
					}
				}()
			}

			codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
			fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), codecName)

			// appSrc := pipelineForCodec(track, codecName)
			buf := make([]byte, 1400)
			for {
				i, _, readErr := track.Read(buf)
				if readErr != nil {
					log.Log(ctx, "failed to read track", "error", readErr)
					cancel()
				}
				log.Log(ctx, "read track", "bytes", i)

				// appSrc.PushBuffer(gst.NewBufferFromBytes(buf[:i]))
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
