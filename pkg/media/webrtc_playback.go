package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
	"stream.place/streamplace/pkg/spmetrics"
)

// we have a bug that prevents us from correctly probing video durations
// a lot of the time. so when we don't have them we use the last duration
// that we had, and when we don't have that we use a default duration
var DefaultDuration = time.Duration(32 * time.Millisecond)

// This function remains in scope for the duration of a single users' playback
func (mm *MediaManager) WebRTCPlayback(ctx context.Context, user string, rendition string, offer *webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "webrtcID", uu.String())
	ctx = log.WithLogValues(ctx, "mediafunc", "WebRTCPlayback")
	ctx, cancel := context.WithCancel(ctx) //nolint:all

	pipelineSlice := []string{
		"h264parse name=videoparse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"opusparse name=audioparse ! appsink name=audioappsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to create GStreamer pipeline: %w", err) //nolint:all
	}

	segBuffer := make(chan *segchanman.Seg, 1024)
	go func() {
		ch := mm.SubscribeSegment(ctx, user, rendition)
		defer mm.UnsubscribeSegment(ctx, user, rendition, ch)
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case file := <-ch:
				log.Debug(ctx, "got segment", "file", file.Filepath)
				segBuffer <- file
			}
		}
	}()

	segCh := make(chan *segchanman.Seg)
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case seg := <-segBuffer:
				select {
				case <-ctx.Done():
					return
				case segCh <- seg:
				}
			}
		}
	}()

	concatBin, err := ConcatBin(ctx, segCh)
	if err != nil {
		return nil, fmt.Errorf("failed to create concat bin: %w", err)
	}

	err = pipeline.Add(concatBin.Element)
	if err != nil {
		return nil, fmt.Errorf("failed to add concat bin to pipeline: %w", err)
	}

	videoPad := concatBin.GetStaticPad("video_0")
	if videoPad == nil {
		return nil, fmt.Errorf("video pad not found")
	}

	audioPad := concatBin.GetStaticPad("audio_0")
	if audioPad == nil {
		return nil, fmt.Errorf("audio pad not found")
	}

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
	videoParsePad := videoParse.GetStaticPad("sink")
	if videoParsePad == nil {
		return nil, fmt.Errorf("video parse pad not found")
	}
	linked := videoPad.Link(videoParsePad)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link video pad to video parse pad: %v", linked)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return nil, fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	audioParsePad := audioParse.GetStaticPad("sink")
	if audioParsePad == nil {
		return nil, fmt.Errorf("audio parse pad not found")
	}
	linked = audioPad.Link(audioParsePad)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link audio pad to audio parse pad: %v", linked)
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

	var lastVideoDuration = &DefaultDuration

	go func() {
		go func() {
			if err := HandleBusMessages(ctx, pipeline); err != nil {
				log.Log(ctx, "pipeline error", "error", err)
			}
			cancel()
		}()

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
				b2 := make([]byte, len(samples))
				copy(b2, samples)
				clockTime := buffer.Duration()
				dur := clockTime.AsDuration()
				mediaSample := media.Sample{Data: b2}
				if dur != nil {
					mediaSample.Duration = *dur
					lastVideoDuration = dur
				} else if lastVideoDuration != nil {
					mediaSample.Duration = *lastVideoDuration
				} else {
					log.Log(ctx, "no video duration", "samples", len(b2))
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

				b2 := make([]byte, len(samples))
				copy(b2, samples)

				clockTime := buffer.Duration()
				dur := clockTime.AsDuration()
				mediaSample := media.Sample{Data: b2}
				if dur != nil {
					mediaSample.Duration = *dur
				} else {
					log.Log(ctx, "no audio duration", "samples", len(b2))
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
		err := pipeline.SetState(gst.StatePlaying)
		if err != nil {
			log.Log(ctx, "failed to set pipeline state to null", "error", err)
		}
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
		})

		// Set the handler for Peer connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
			log.Log(ctx, "Peer Connection State has changed", "state", s.String())

			if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateDisconnected {
				// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
				// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
				// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
				log.Log(ctx, "Peer Connection has gone to failed, exiting")
				cancel()
			}
		})

		<-ctx.Done()

		log.Warn(ctx, "setting playback pipeline state to null")
		err = pipeline.BlockSetState(gst.StateNull)
		if err != nil {
			log.Log(ctx, "failed to set pipeline state to null", "error", err)
		}

		videoappsink.SetCallbacks(&app.SinkCallbacks{})
		err = videoappsinkele.SetState(gst.StateNull)
		if err != nil {
			log.Log(ctx, "failed to set videoappsinkele state to null", "error", err)
		}

		audioappsink.SetCallbacks(&app.SinkCallbacks{})
		err = audioappsinkele.SetState(gst.StateNull)
		if err != nil {
			log.Log(ctx, "failed to set audioappsinkele state to null", "error", err)
		}

		log.Warn(ctx, "exiting playback")

	}()
	select {
	case <-gatherComplete:
		return peerConnection.LocalDescription(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
