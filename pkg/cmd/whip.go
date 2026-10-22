package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/abema/go-mp4"
	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func WHIP(ctx context.Context, streamKey string, count int, viewers int, duration time.Duration, file string, endpoint string, freezeAfter time.Duration, retry time.Duration) error {
	if file == "" {
		return fmt.Errorf("file is required")
	}
	if retry < 0 {
		return fmt.Errorf("retry must not be negative")
	}
	gstinit.InitGST()

	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	w := &WHIPClient{
		StreamKey:   streamKey,
		File:        file,
		Endpoint:    endpoint,
		Count:       count,
		FreezeAfter: freezeAfter,
		Viewers:     viewers,
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := w.WHIP(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if retry == 0 {
			return err
		}
		log.Log(ctx, "WHIP session ended, retrying", "error", err, "delay", retry)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retry):
		}
	}
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

func (w *WHIPClient) WHIP(ctx context.Context) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	codec := fileVideoCodec(w.File)
	pipelineSlice := []string{
		"filesrc name=filesrc ! qtdemux name=demux",
		"demux.video_0 ! tee name=video_tee",
		"demux.audio_0 ! tee name=audio_tee",
		// sync=true (the default) is load-bearing here, unlike every other
		// appsink in the tree: these sinks are what pace the file at realtime
		// so it plays as a live stream — WriteSample pushes to WebRTC
		// immediately, so without clock sync the whole file would blast
		// through in one burst.
		fmt.Sprintf("video_tee. ! queue ! %s ! %s ! appsink sync=true name=videoappsink", codec.ParserLaunch("", "config-interval=-1"), codec.SampleCaps),
		"audio_tee. ! queue ! opusparse ! appsink sync=true name=audioappsink",
		// "matroskamux name=mux ! fakesink name=fakesink sync=true",
		// "video_tee. ! mux.video_0",
		// "audio_tee. ! mux.audio_0",
	}

	element, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return err
	}
	pipeline := element.(gst.Pipeline)
	defer func() {
		cancel()
		if status := pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone); status != gst.StateChangeSuccess && retErr == nil {
			retErr = fmt.Errorf("stop WHIP pipeline: %s", status)
		}
	}()

	fileSrc := pipeline.GetByName("filesrc")
	if fileSrc == nil {
		return fmt.Errorf("filesrc not found")
	}
	fileSrc.SetObjectProperty("location", w.File)

	videoSink := pipeline.GetByName("videoappsink")
	if videoSink == nil {
		return fmt.Errorf("video appsink not found")
	}

	audioSink := pipeline.GetByName("audioappsink")
	if audioSink == nil {
		return fmt.Errorf("audio appsink not found")
	}

	startTime := time.Now()
	sinks := []app.AppSink{
		videoSink.(app.AppSink),
		audioSink.(app.AppSink),
	}
	// Create accumulators for tracking elapsed duration
	accumulators := make([]time.Duration, len(sinks))

	conns := make([]*WHIPConnection, w.Count)
	defer func() {
		for _, conn := range conns {
			if conn != nil {
				conn.peerConnection.Close()
			}
		}
	}()
	g := &errgroup.Group{}
	for i := 0; i < w.Count; i++ {
		ctx := ctx
		// var streamKey string
		var did string
		var streamKey string
		if w.StreamKey != "" {
			streamKey = w.StreamKey
		} else {
			priv, pub, err := spkey.GenerateStreamKey()
			if err != nil {
				return err
			}

			did = pub.DIDKey()
			ctx = log.WithLogValues(ctx, "did", did)
			streamKey = priv.Multibase()
		}

		g.Go(func() error {
			conn, err := w.StartWHIPConnection(ctx, streamKey, did, codec)
			if err != nil {
				cancel()
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
	reportError := func(err error) {
		select {
		case errCh <- err:
		default:
		}
		cancel()
	}

	for i := range sinks {
		func(i int) {
			sink := sinks[i]
			trackType := "video"
			if i == 1 {
				trackType = "audio"
			}

			sink.SetCallbacks(&app.AppSinkCallbacks{
				NewSample: func(sink app.AppSink) gst.FlowReturn {

					sample := sink.PullSample()
					if sample == nil {
						return gst.FlowEOS
					}

					buffer := sample.GetBuffer()
					if buffer == nil {
						return gst.FlowError
					}

					mapped, ok := buffer.Map(gst.MapRead)
					if !ok {
						reportError(fmt.Errorf("%s buffer mapping failed", trackType))
						return gst.FlowError
					}
					defer mapped.Unmap()
					samples := mapped.Data()
					if trackType == "video" {
						// Pion may retain SPS/PPS slices between H264 samples.
						samples = append([]byte(nil), samples...)
					}

					clockDuration := buffer.Duration()
					if clockDuration == gst.ClockTimeNone {
						reportError(fmt.Errorf("%v duration: nil", trackType))
						return gst.FlowError
					}
					duration := time.Duration(clockDuration)

					accumulators[i] += duration

					if w.FreezeAfter == 0 || time.Since(startTime) < w.FreezeAfter {
						for _, conn := range conns {
							if trackType == "video" {
								if err := conn.videoTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
									log.Log(ctx, "error writing video sample", "error", err)
									reportError(err)
									return gst.FlowError
								}
							} else {
								if err := conn.audioTrack.WriteSample(pionmedia.Sample{Data: samples, Duration: duration}); err != nil {
									log.Log(ctx, "error writing video sample", "error", err)
									reportError(err)
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
		if err := media.HandleBusMessages(ctx, pipeline); err != nil {
			log.Log(ctx, "pipeline error", "error", err)
		}
		cancel()
	}()

	if pipeline.SetState(gst.StatePlaying) == gst.StateChangeFailure {
		return fmt.Errorf("failed to start WHIP pipeline")
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

	// reportError cancels the session context right after queueing the error it
	// saw, so both branches below would be ready at once and the select would
	// pick either one. Prefer the queued error: it is the actual failure, and
	// returning context.Canceled instead would hide it from both the one-shot
	// caller and the retry log.
	select {
	case err := <-errCh:
		return err
	default:
	}
	return ctx.Err()
}

func (w *WHIPClient) StartWHIPConnection(ctx context.Context, streamKey string, did string, codec media.VideoCodec) (*WHIPConnection, error) {

	// Prepare the configuration
	config := webrtc.Configuration{}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}
	connected := false
	defer func() {
		if !connected {
			peerConnection.Close()
		}
	}()

	// Create a audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		return nil, err
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, err
	}

	// Create a video track, offered in its codec only: the server answers
	// H.264 whenever an offer has it.
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: codec.MimeType}, "video", "pion2")
	if err != nil {
		return nil, err
	}
	videoSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		return nil, err
	}
	if err := offerOnly(peerConnection, videoSender, codec.MimeType); err != nil {
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
	req, err := http.NewRequestWithContext(ctx, "POST", w.Endpoint, strings.NewReader(offer.SDP))
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
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-gatherComplete:
	}

	conn := &WHIPConnection{
		peerConnection: peerConnection,
		audioTrack:     audioTrack,
		videoTrack:     videoTrack,
		did:            did,
	}

	connected = true
	return conn, nil
}

// offerOnly narrows the offer for sender's track to the codecs of mimeType.
func offerOnly(pc *webrtc.PeerConnection, sender *webrtc.RTPSender, mimeType string) error {
	var keep []webrtc.RTPCodecParameters
	for _, c := range sender.GetParameters().Codecs {
		if strings.EqualFold(c.MimeType, mimeType) {
			keep = append(keep, c)
		}
	}
	for _, t := range pc.GetTransceivers() {
		if t.Sender() == sender {
			return t.SetCodecPreferences(keep)
		}
	}
	return fmt.Errorf("no transceiver for the %s track", mimeType)
}

// fileVideoCodec is the codec of an MP4 file's first video track: its first
// sample entry in a codec Streamplace knows. H.264 when it can't tell; a file
// that can't be read fails in the pipeline, which says why.
func fileVideoCodec(path string) media.VideoCodec {
	f, err := os.Open(path)
	if err != nil {
		return media.VideoH264
	}
	defer f.Close()
	entries, _ := mp4.ExtractBox(f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeAny()})
	for _, e := range entries {
		if c, ok := media.VideoCodecForCatalog(e.Type.String()); ok {
			return c
		}
	}
	return media.VideoH264
}
