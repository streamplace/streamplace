package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func WHIP(args []string) error {
	fs := flag.NewFlagSet("whip", flag.ExitOnError)
	streamKey := fs.String("stream-key", "", "stream key")
	count := fs.Int("count", 1, "number of concurrent streams (for load testing)")
	viewers := fs.Int("viewers", 0, "number of viewers to simulate per stream")
	duration := fs.Duration("duration", 0, "duration of the stream")
	file := fs.String("file", "", "file to stream (needs to be an MP4 containing H264 video and Opus audio)")
	endpoint := fs.String("endpoint", "http://127.0.0.1:38080", "endpoint to send the WHIP request to")
	freezeAfter := fs.Duration("freeze-after", 0, "freeze the stream after the given duration")
	err := fs.Parse(args)
	if *file == "" {
		return fmt.Errorf("file is required")
	}
	if err != nil {
		return err
	}
	gstinit.InitGST()

	ctx := context.Background()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	w := &WHIPClient{
		StreamKey:   *streamKey,
		File:        *file,
		Endpoint:    *endpoint,
		Count:       *count,
		FreezeAfter: *freezeAfter,
		Viewers:     *viewers,
	}

	return w.WHIP(ctx)
}

type WHIPClient struct {
	StreamKey   string
	File        string
	Endpoint    string
	Count       int
	FreezeAfter time.Duration
	Viewers     int
}

var failureStates = []webrtc.ICEConnectionState{
	webrtc.ICEConnectionStateFailed,
	webrtc.ICEConnectionStateDisconnected,
	webrtc.ICEConnectionStateClosed,
	webrtc.ICEConnectionStateCompleted,
}

type WHIPConnection struct {
	peerConnection *webrtc.PeerConnection
	audioTrack     *webrtc.TrackLocalStaticSample
	videoTrack     *webrtc.TrackLocalStaticSample
	did            string
}

func (w *WHIPClient) WHIP(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"filesrc name=filesrc ! qtdemux name=demux",
		"demux.video_0 ! tee name=video_tee",
		"demux.audio_0 ! tee name=audio_tee",
		"video_tee. ! queue ! h264parse ! video/x-h264,stream-format=byte-stream ! appsink name=videoappsink",
		"audio_tee. ! queue ! opusparse ! appsink name=audioappsink",
		// "matroskamux name=mux ! fakesink name=fakesink sync=true",
		// "video_tee. ! mux.video_0",
		// "audio_tee. ! mux.audio_0",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return err
	}

	fileSrc, err := pipeline.GetElementByName("filesrc")
	if err != nil {
		return err
	}

	fileSrc.Set("location", w.File)

	videoSink, err := pipeline.GetElementByName("videoappsink")
	if err != nil {
		return err
	}

	audioSink, err := pipeline.GetElementByName("audioappsink")
	if err != nil {
		return err
	}

	startTime := time.Now()
	sinks := []*app.Sink{
		app.SinkFromElement(videoSink),
		app.SinkFromElement(audioSink),
	}
	// Create accumulators for tracking elapsed duration
	accumulators := make([]time.Duration, len(sinks))

	conns := make([]*WHIPConnection, w.Count)
	g := &errgroup.Group{}
	for i := 0; i < w.Count; i++ {
		ctx := ctx
		// var streamKey string
		var did string
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

			did = pub.DIDKey()
			ctx = log.WithLogValues(ctx, "did", did)
			streamKey = priv.Multibase()
		}

		g.Go(func() error {
			conn, err := w.StartWHIPConnection(ctx, streamKey, did)
			if err != nil {
				return err
			}
			conns[i] = conn
			ctx := log.WithLogValues(ctx, "did", did)
			conn.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
				log.Log(ctx, "WHIP connection State has changed", "state", connectionState.String())
				for _, state := range failureStates {
					if connectionState == state {
						log.Log(ctx, "connection failed, cancelling")
						cancel()
					}
				}
			})
			go func() {
				<-ctx.Done()
				if conn.peerConnection != nil {
					conn.peerConnection.Close()
				}
			}()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Start a ticker to print elapsed duration every second
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
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
		}
	}()

	errCh := make(chan error, 1)

	for i, _ := range sinks {
		func(i int) {
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

					if w.FreezeAfter == 0 || time.Since(startTime) < w.FreezeAfter {
						for _, conn := range conns {
							if trackType == "video" {
								if err := conn.videoTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
									log.Log(ctx, "error writing video sample", "error", err)
									errCh <- err
									return gst.FlowError
								}
							} else {
								if err := conn.audioTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
									log.Log(ctx, "error writing video sample", "error", err)
									errCh <- err
									return gst.FlowError
								}
							}
						}
					}

					return gst.FlowOK
				},
			})
		}(i)
	}

	go func() {
		media.HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	if err = pipeline.SetState(gst.StatePlaying); err != nil {
		return err
	}
	if w.Viewers > 0 {
		whepG, ctx := errgroup.WithContext(ctx)
		for i := 0; i < w.Count; i++ {
			did := conns[i].did
			w := &WHEPClient{
				Endpoint: fmt.Sprintf("%s/api/playback/%s/webrtc", w.Endpoint, did),
				Count:    w.Viewers,
			}
			whepG.Go(func() error {
				return w.WHEP(ctx)
			})
		}
		if err := whepG.Wait(); err != nil {
			return err
		}
	}

	<-ctx.Done()
	err = pipeline.BlockSetState(gst.StateNull)
	if err != nil {
		return err
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *WHIPClient) StartWHIPConnection(ctx context.Context, streamKey string, did string) (*WHIPConnection, error) {

	// Prepare the configuration
	config := webrtc.Configuration{}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		return nil, err
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, err
	}

	// Create a video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion2")
	if err != nil {
		return nil, err
	}
	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		return nil, err
	}

	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	// Set the generated offer as our LocalDescription
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return nil, err
	}

	// Wait for ICE gathering to complete
	// gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	// <-gatherComplete

	// Create HTTP client and prepare the request
	client := &http.Client{}

	// Send the WHIP request to the server
	req, err := http.NewRequest("POST", w.Endpoint, strings.NewReader(offer.SDP))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+streamKey)
	req.Header.Set("Content-Type", "application/sdp")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and process the answer
	answerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the SDP answer
	var answer webrtc.SessionDescription
	answer.Type = webrtc.SDPTypeAnswer
	answer.SDP = string(answerBytes)

	// Apply the answer as remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	conn := &WHIPConnection{
		peerConnection: peerConnection,
		audioTrack:     audioTrack,
		videoTrack:     videoTrack,
		did:            did,
	}

	return conn, nil
}
