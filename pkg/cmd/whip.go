// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// gstreamer-send is a simple application that shows how to send video to your browser using Pion WebRTC and GStreamer.
package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"stream.place/streamplace/pkg/log"
)

func WHIP() error {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	// audioSrc := flag.String("audio-src", "audiotestsrc", "GStreamer audio src")
	// videoSrc := flag.String("video-src", "videotestsrc", "GStreamer video src")
	// flag.Parse()

	// Initialize GStreamer
	gst.Init(nil)

	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		panic(err)
	}

	// Create a video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion2")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

	// Create an answer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(offer.SDP)

	// Set the generated offer as our LocalDescription
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// Wait for ICE gathering to complete
	// gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	// <-gatherComplete

	// Create HTTP client and prepare the request
	client := &http.Client{}

	// Send the WHIP request to the server
	req, err := http.NewRequest("POST", "http://127.0.0.1:38080", strings.NewReader(offer.SDP))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Authorization", "Bearer zEaiwgbN8uRT9jKqVsw3ZzqQunsqUHqxJE9ZffPHBNnSx")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read and process the answer
	answerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// Parse the SDP answer
	var answer webrtc.SessionDescription
	answer.Type = webrtc.SDPTypeAnswer
	answer.SDP = string(answerBytes)

	// Apply the answer as remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		panic(err)
	}

	// // Wait for the offer to be pasted
	// offer := webrtc.SessionDescription{}
	// decode(readUntilNewline(), &offer)

	// // Set the remote SessionDescription
	// err = peerConnection.SetRemoteDescription(offer)
	// if err != nil {
	// 	panic(err)
	// }

	// // Create an answer
	// answer, err := peerConnection.CreateAnswer(nil)
	// if err != nil {
	// 	panic(err)
	// }

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// // Sets the LocalDescription, and starts our UDP listeners
	// err = peerConnection.SetLocalDescription(answer)
	// if err != nil {
	// 	panic(err)
	// }

	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	// fmt.Println(encode(peerConnection.LocalDescription()))

	// switch codecName {
	// case "vp8":
	// 	pipelineStr = pipelineSrc + " ! vp8enc error-resilient=partitions keyframe-max-dist=10 auto-alt-ref=true cpu-used=5 deadline=1 ! " + pipelineStr
	// case "vp9":
	// 	pipelineStr = pipelineSrc + " ! vp9enc ! " + pipelineStr
	// case "h264":
	// 	pipelineStr = pipelineSrc + " ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast tune=zerolatency key-int-max=20 ! video/x-h264,stream-format=byte-stream ! " + pipelineStr
	// case "opus":
	// 	pipelineStr = pipelineSrc + " ! opusenc ! " + pipelineStr
	// case "pcmu":
	// 	pipelineStr = pipelineSrc + " ! audio/x-raw, rate=8000 ! mulawenc ! " + pipelineStr
	// case "pcma":
	// 	pipelineStr = pipelineSrc + " ! audio/x-raw, rate=8000 ! alawenc ! " + pipelineStr
	// default:
	// 	panic("Unhandled codec " + codecName) //nolint
	// }

	pipelineSlice := []string{
		"filesrc location=/home/iameli/testvids/RocketLeague_1h55m_1sGOP_1080p60_NoBframes.mp4 ! qtdemux name=demux",
		"demux.video_0 ! tee name=video_tee",
		"demux.audio_0 ! tee name=audio_tee",
		"video_tee. ! queue ! h264parse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"audio_tee. ! queue ! opusparse ! rtpopuspay ! appsink name=audioappsink",
		// "matroskamux name=mux ! fakesink name=fakesink sync=true",
		// "video_tee. ! mux.video_0",
		// "audio_tee. ! mux.audio_0",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		panic(err)
	}

	videoSink, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		panic(err)
	}

	audioSink, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		panic(err)
	}

	startTime := time.Now()
	tracks := []*webrtc.TrackLocalStaticSample{
		videoTrack,
		audioTrack,
	}
	sinks := []*app.Sink{
		app.SinkFromElement(videoSink),
		app.SinkFromElement(audioSink),
	}
	// Create accumulators for tracking elapsed duration
	accumulators := make([]time.Duration, len(tracks))

	// Start a ticker to print elapsed duration every second
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			for i, duration := range accumulators {
				trackType := "video"
				if i == 1 {
					trackType = "audio"
				}
				target := startTime.Add(time.Duration(accumulators[i]))
				diff := time.Since(target)
				fmt.Printf("%v elapsed duration: %v diff from real-time: %v\n", trackType, duration, diff)
			}
		}
	}()

	for i, track := range tracks {
		func(i int, track *webrtc.TrackLocalStaticSample) {
			sink := sinks[i]
			trackType := "video"
			if i == 1 {
				trackType = "audio"
			}

			sink.SetCallbacks(&app.SinkCallbacks{
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

					durationPtr := buffer.Duration().AsDuration()
					var duration time.Duration
					if durationPtr == nil {
						// fmt.Printf("%v duration: nil\n", trackType)
						panic(fmt.Sprintf("%v duration: nil\n", trackType))
						duration = 32 * time.Millisecond
					} else {
						// fmt.Printf("%v duration: %v\n", trackType, *durationPtr)
						duration = *durationPtr
					}

					accumulators[i] += duration

					if err := track.WriteSample(media.Sample{Data: samples, Duration: duration}); err != nil {
						panic(err) //nolint
					}

					return gst.FlowOK
				},
			})
		}(i, track)
	}

	ok := pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
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
	if !ok {
		return fmt.Errorf("failed to add watch to pipeline bus")
	}

	if err = pipeline.SetState(gst.StatePlaying); err != nil {
		panic(err)
	}

	select {}

	return nil
}

// Read from stdin until we get a newline
func readUntilNewline() (in string) {
	var err error

	r := bufio.NewReader(os.Stdin)
	for {
		in, err = r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}

		if in = strings.TrimSpace(in); len(in) > 0 {
			break
		}
	}

	fmt.Println("")
	return in
}

// JSON encode + base64 a SessionDescription
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}
