// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// gstreamer-send is a simple application that shows how to send video to your browser using Pion WebRTC and GStreamer.
package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

func WHIP() error {
	fs := flag.NewFlagSet("whip", flag.ExitOnError)
	streamKey := fs.String("stream-key", "", "stream key")
	count := fs.Int("count", 1, "number of concurrent streams (for load testing)")
	duration := fs.Duration("duration", 0, "duration of the stream")
	err := fs.Parse(os.Args[2:])
	if err != nil {
		return err
	}

	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	for i := 0; i < *count; i++ {
		w := &WHIPClient{
			StreamKey: *streamKey,
		}
		g.Go(func() error {
			return w.WHIP(ctx)
		})
	}

	return g.Wait()
}

type WHIPClient struct {
	StreamKey string
}

var failureStates = []webrtc.ICEConnectionState{
	webrtc.ICEConnectionStateFailed,
	webrtc.ICEConnectionStateDisconnected,
	webrtc.ICEConnectionStateClosed,
	webrtc.ICEConnectionStateCompleted,
}

func (w *WHIPClient) WHIP(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	// audioSrc := flag.String("audio-src", "audiotestsrc", "GStreamer audio src")
	// videoSrc := flag.String("video-src", "videotestsrc", "GStreamer video src")
	// flag.Parse()

	var streamKey string
	if w.StreamKey != "" {
		streamKey = w.StreamKey
	} else {
		priv, err := atcrypto.GeneratePrivateKeyK256()
		if err != nil {
			return err
		}
		pub, err := priv.PublicKey()
		if err != nil {
			return err
		}

		did := pub.DIDKey()
		ctx = log.WithLogValues(ctx, "did", did)
		streamKey = priv.Multibase()
	}

	// Initialize GStreamer
	gst.Init(nil)

	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Log(ctx, "connection State has changed", "state", connectionState.String())
		for _, state := range failureStates {
			if connectionState == state {
				cancel()
			}
		}
	})

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		return err
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		return err
	}

	// Create a video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion2")
	if err != nil {
		return err
	}
	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		return err
	}

	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	fmt.Println(offer.SDP)

	// Set the generated offer as our LocalDescription
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return err
	}

	// Wait for ICE gathering to complete
	// gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	// <-gatherComplete

	// Create HTTP client and prepare the request
	client := &http.Client{}

	// Send the WHIP request to the server
	req, err := http.NewRequest("POST", "http://127.0.0.1:38080", strings.NewReader(offer.SDP))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+streamKey)
	req.Header.Set("Content-Type", "application/sdp")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read and process the answer
	answerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse the SDP answer
	var answer webrtc.SessionDescription
	answer.Type = webrtc.SDPTypeAnswer
	answer.SDP = string(answerBytes)

	// Apply the answer as remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

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
		return err
	}

	videoSink, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return err
	}

	audioSink, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return err
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
				log.Debug(ctx, "elapsed duration", "track", trackType, "duration", duration, "diff", diff)
			}
		}
	}()

	errCh := make(chan error, 1)

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
						errCh <- fmt.Errorf("%v duration: nil", trackType)
						return gst.FlowError
					} else {
						// fmt.Printf("%v duration: %v\n", trackType, *durationPtr)
						duration = *durationPtr
					}

					accumulators[i] += duration

					if err := track.WriteSample(media.Sample{Data: samples, Duration: duration}); err != nil {
						errCh <- err
						return gst.FlowError
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
		return err
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
