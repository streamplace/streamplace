package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

func WHEP(args []string) error {
	fs := flag.NewFlagSet("whep", flag.ExitOnError)
	count := fs.Int("count", 1, "number of concurrent streams (for load testing)")
	duration := fs.Duration("duration", 0, "stop after this long")
	endpoint := fs.String("endpoint", "", "endpoint to send the WHEP request to")
	err := fs.Parse(args)

	if err != nil {
		return err
	}

	ctx := context.Background()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	w := &WHEPClient{
		Endpoint: *endpoint,
		Count:    *count,
	}

	return w.WHEP(ctx)
}

type WHEPClient struct {
	StreamKey   string
	File        string
	Endpoint    string
	Count       int
	FreezeAfter time.Duration
}

type WHEPConnection struct {
	peerConnection *webrtc.PeerConnection
	audioTrack     *webrtc.TrackLocalStaticSample
	videoTrack     *webrtc.TrackLocalStaticSample
	did            string
	Done           func() <-chan struct{}
}

func (w *WHEPClient) StartWHEPConnection(ctx context.Context) (*WHEPConnection, error) {

	// Prepare the configuration
	config := webrtc.Configuration{}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Track statistics
	type trackStats struct {
		total      int
		lastTotal  int
		lastUpdate time.Time
		mu         sync.Mutex
	}

	stats := map[string]*trackStats{
		"video": {lastUpdate: time.Now()},
		"audio": {lastUpdate: time.Now()},
	}

	// Create a ticker to print combined bitrate every 5 seconds
	ticker := time.NewTicker(5 * time.Second)

	// Start a goroutine to print combined bitrate
	go func() {
		for {
			select {
			case <-ticker.C:
				currentTime := time.Now()

				// Lock both stats to get a consistent snapshot
				for _, s := range stats {
					s.mu.Lock()
				}

				videoStats := stats["video"]
				audioStats := stats["audio"]

				videoElapsed := currentTime.Sub(videoStats.lastUpdate).Seconds()
				audioElapsed := currentTime.Sub(audioStats.lastUpdate).Seconds()

				videoBytes := videoStats.total - videoStats.lastTotal
				audioBytes := audioStats.total - audioStats.lastTotal

				videoBitrate := float64(videoBytes) * 8 / videoElapsed / 1000 // kbps
				audioBitrate := float64(audioBytes) * 8 / audioElapsed / 1000 // kbps

				log.Log(ctx, "bitrate stats",
					"video", fmt.Sprintf("%.2f kbps (%.2f KB)", videoBitrate, float64(videoBytes)/1000),
					"audio", fmt.Sprintf("%.2f kbps (%.2f KB)", audioBitrate, float64(audioBytes)/1000),
					"total", fmt.Sprintf("%.2f kbps", videoBitrate+audioBitrate))

				// Update last values
				videoStats.lastTotal = videoStats.total
				videoStats.lastUpdate = currentTime
				audioStats.lastTotal = audioStats.total
				audioStats.lastUpdate = currentTime

				// Unlock stats
				for _, s := range stats {
					s.mu.Unlock()
				}

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			log.Log(ctx, "track received", "track", track.ID())

			// Determine track type
			trackType := "video"
			if track.Kind() == webrtc.RTPCodecTypeAudio {
				trackType = "audio"
			}

			trackStat := stats[trackType]

			for {
				if ctx.Err() != nil {
					return
				}
				rtp, _, err := track.ReadRTP()
				if err != nil {
					log.Log(ctx, "error reading RTP", "error", err)
					cancel()
					return
				}

				trackStat.mu.Lock()
				trackStat.total += len(rtp.Payload)
				trackStat.mu.Unlock()
			}
		})
		peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			log.Log(ctx, "WHEP connection State has changed", "state", connectionState.String())
			for _, state := range failureStates {
				if connectionState == state {
					log.Log(ctx, "connection failed, cancelling")
					cancel()
				}
			}
		})

		<-ctx.Done()
		peerConnection.Close()
	}()
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Log(ctx, "ICE candidate", "candidate", candidate)
	})
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		return nil, fmt.Errorf("failed to add video transceiver: %w", err)
	}
	if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		return nil, fmt.Errorf("failed to add audio transceiver: %w", err)
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
	req.Header.Set("Content-Type", "application/sdp")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

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

	conn := &WHEPConnection{
		peerConnection: peerConnection,
		Done:           ctx.Done,
	}

	return conn, nil
}

func (w *WHEPClient) WHEP(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conns := make([]*WHEPConnection, w.Count)
	g := &errgroup.Group{}
	for i := 0; i < w.Count; i++ {
		g.Go(func() error {
			conn, err := w.StartWHEPConnection(ctx)
			if err != nil {
				return err
			}
			conns[i] = conn

			<-conn.Done()

			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return err
	}
	// if err := g.Wait(); err != nil {
	// if err := g.Wait(); err != nil {
	// 	return err
	// }
	// // Start a ticker to print elapsed duration every second
	// go func() {
	// 	ticker := time.NewTicker(time.Second)
	// 	defer ticker.Stop()

	// 	for {
	// 		<-ticker.C
	// 		for i, duration := range accumulators {
	// 			trackType := "video"
	// 			if i == 1 {
	// 				trackType = "audio"
	// 			}
	// 			target := startTime.Add(time.Duration(accumulators[i]))
	// 			diff := time.Since(target)
	// 			log.Debug(ctx, "elapsed duration", "track", trackType, "duration", duration, "diff", diff)
	// 		}
	// 	}
	// }()

	// errCh := make(chan error, 1)

	// for i, _ := range sinks {
	// 	func(i int) {
	// 		sink := sinks[i]
	// 		trackType := "video"
	// 		if i == 1 {
	// 			trackType = "audio"
	// 		}

	// 		sink.SetCallbacks(&app.SinkCallbacks{
	// 			NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {

	// 				sample := sink.PullSample()
	// 				if sample == nil {
	// 					return gst.FlowEOS
	// 				}

	// 				buffer := sample.GetBuffer()
	// 				if buffer == nil {
	// 					return gst.FlowError
	// 				}

	// 				samples := buffer.Map(gst.MapRead).Bytes()
	// 				defer buffer.Unmap()

	// 				durationPtr := buffer.Duration().AsDuration()
	// 				var duration time.Duration
	// 				if durationPtr == nil {
	// 					errCh <- fmt.Errorf("%v duration: nil", trackType)
	// 					return gst.FlowError
	// 				} else {
	// 					// fmt.Printf("%v duration: %v\n", trackType, *durationPtr)
	// 					duration = *durationPtr
	// 				}

	// 				accumulators[i] += duration

	// 				if w.FreezeAfter == 0 || time.Since(startTime) < w.FreezeAfter {
	// 					for _, conn := range conns {
	// 						if trackType == "video" {
	// 							if err := conn.videoTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
	// 								log.Log(ctx, "error writing video sample", "error", err)
	// 								errCh <- err
	// 								return gst.FlowError
	// 							}
	// 						} else {
	// 							if err := conn.audioTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
	// 								log.Log(ctx, "error writing video sample", "error", err)
	// 								errCh <- err
	// 								return gst.FlowError
	// 							}
	// 						}
	// 					}
	// 				}

	// 				return gst.FlowOK
	// 			},
	// 		})
	// 	}(i)
	// }

	// go func() {
	// 	media.HandleBusMessages(ctx, pipeline)
	// 	cancel()
	// }()

	// if err = pipeline.SetState(gst.StatePlaying); err != nil {
	// 	return err
	// }
	// select {
	// case err := <-errCh:
	// 	return err
	// case <-ctx.Done():
	// 	return ctx.Err()
	// }

	return nil
}
