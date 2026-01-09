package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/muxionlabs/ai-go-sdk/pkg/client"
	"stream.place/streamplace/pkg/log"
)

type AISample struct {
	Data []byte
	Dur  time.Duration
}

type AISessionResources struct {
	VideoCh chan AISample
	AudioCh chan AISample
	Cleanup func()
}

func (mm *MediaManager) aiGatewayStreamConfig() client.StreamConfig {
	enableVideoIngress := true
	enableVideoEgress := false
	enableDataOutput := true

	return client.StreamConfig{
		BaseURL:            mm.cli.AIGatewayBaseURL,
		Pipeline:           mm.cli.AIGatewayPipeline,
		EnableVideoIngress: &enableVideoIngress,
		EnableVideoEgress:  &enableVideoEgress,
		EnableDataOutput:   &enableDataOutput,
	}
}

// SetupAISession initializes an AI session, WHIP publisher, and handling channels.
func (mm *MediaManager) SetupAISession(
	ctx context.Context,
	streamer string,
	onTranscript func(context.Context, client.TranscriptEvent),
) (*AISessionResources, error) {
	if mm.cli.AIGatewayBaseURL == "" {
		return nil, fmt.Errorf("AI gateway not configured")
	}

	cfg := mm.aiGatewayStreamConfig()
	logger := newAIGatewayLogger()

	streamName := fmt.Sprintf("streamplace-%s-%d", streamer, time.Now().UnixMilli())

	session, err := client.StartSession(ctx, cfg, streamName)
	if err != nil {
		log.Error(ctx, "failed to start AI gateway session", "error", err)
		return nil, err
	}

	log.Debug(ctx, "AI gateway session started",
		"streamID", session.ID,
		"dataURL", session.DataStreamURL,
		"streamer", streamer,
	)

	if session.WHIPURL == "" {
		log.Error(ctx, "AI gateway did not provide whip_url", "streamID", session.ID)
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return nil, fmt.Errorf("no WHIP URL provided")
	}

	wrappedOnTranscript := func(ctx context.Context, event client.TranscriptEvent) {
		mm.transcriptStore.AddEvent(ctx, streamer, event)
		if onTranscript != nil {
			onTranscript(ctx, event)
		}
	}

	mm.startTranscriptSSE(ctx, session.DataStreamURL, streamer, logger, wrappedOnTranscript)

	publisher := client.NewWHIPPublisher(ctx, session.WHIPURL, logger)
	if err := publisher.Start(); err != nil {
		log.Error(ctx, "failed to start WHIP publisher", "error", err)
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return nil, err
	}

	videoCh := make(chan AISample, 64)
	audioCh := make(chan AISample, 64)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-videoCh:
				if err := publisher.WriteVideoSample(s.Data, s.Dur); err != nil {
					log.Debug(ctx, "WHIP write video sample error", "error", err)
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-audioCh:
				if err := publisher.WriteAudioSample(s.Data, s.Dur); err != nil {
					log.Debug(ctx, "WHIP write audio sample error", "error", err)
				}
			}
		}
	}()

	cleanup := func() {
		publisher.Stop()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StopSession(stopCtx, cfg, session.ID); err != nil {
			log.Error(ctx, "failed to stop AI gateway session", "error", err)
		} else {
			log.Log(ctx, "AI gateway session stopped", "streamID", session.ID)
		}
		mm.transcriptStore.Clear(streamer)
	}

	return &AISessionResources{
		VideoCh: videoCh,
		AudioCh: audioCh,
		Cleanup: cleanup,
	}, nil
}

// NewAISinkCallback creates a GStreamer AppSink callback that pushes samples to the provided channel.
func (mm *MediaManager) NewAISinkCallback(ch chan<- AISample) *app.SinkCallbacks {
	return &app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowEOS
		}
		buf := sample.GetBuffer()
		if buf == nil {
			return gst.FlowError
		}
		b := buf.Map(gst.MapRead).Bytes()
		cpy := make([]byte, len(b))
		copy(cpy, b)
		buf.Unmap()
		durPtr := buf.Duration().AsDuration()
		dur := time.Duration(0)
		if durPtr != nil {
			dur = *durPtr
		}
		select {
		case ch <- AISample{Data: cpy, Dur: dur}:
		default:
		}
		return gst.FlowOK
	}}
}

// PublishTranscriptToBus publishes a transcript event to the event bus.
func (mm *MediaManager) PublishTranscriptToBus(ctx context.Context, streamer string, event client.TranscriptEvent) {
	if mm.bus != nil {
		for _, seg := range event.Segments {
			msg := map[string]any{
				"$type":   "place.stream.ai#dataOutput",
				"id":      seg.ID,
				"text":    seg.Text,
				"startMs": seg.StartMS,
				"endMs":   seg.EndMS,
				"words":   seg.Words,
			}
			mm.bus.Publish(streamer, msg)
		}
	}
}

// StartAISessionFromMKV mirrors the legacy RTMP ingest tee: it demuxes MKV
// (H264+AAC) and forwards samples to the AI gateway via WHIP while streaming
// transcript events through the provided callback.
func (mm *MediaManager) StartAISessionFromMKV(
	ctx context.Context,
	input io.Reader,
	streamer string,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func(), bool) {
	resources, err := mm.SetupAISession(ctx, streamer, onTranscript)
	if err != nil {
		log.Log(ctx, "continuing without AI transcription", "reason", err)
		return input, func() {}, false
	}

	// Helper to start the pipeline and connect it to channels
	teedInput := mm.startAIGatewayPipelineMKV(ctx, input, resources)

	return teedInput, resources.Cleanup, true
}

// startAIGatewayPipelineMKV sets up the GStreamer pipeline for MKV (H264+AAC) ingest
// feeding into the provided AISessionResources channels.
func (mm *MediaManager) startAIGatewayPipelineMKV(
	ctx context.Context,
	input io.Reader,
	res *AISessionResources,
) io.Reader {
	pr, pw := io.Pipe()
	asyncWriter := client.NewAsyncWriter(ctx, pw, newAIGatewayLogger())
	teedInput := io.TeeReader(input, asyncWriter)

	// Wrap cleanup to close asyncWriter
	originalCleanup := res.Cleanup
	res.Cleanup = func() {
		if asyncWriter != nil {
			written, dropped := asyncWriter.Stats()
			log.Log(ctx, "AI gateway tee stats", "written", written, "dropped", dropped)
			_ = asyncWriter.Close()
		}
		originalCleanup()
	}

	go func() {
		pipelineSlice := []string{
			"appsrc name=aisrc ! matroskademux name=demux",
			"demux. ! queue ! h264parse config-interval=-1 ! video/x-h264,stream-format=byte-stream ! appsink sync=false max-buffers=1 drop=true name=videoappsink",
			"demux. ! queue ! fdkaacdec ! audioresample ! audioconvert ! opusenc ! appsink sync=false max-buffers=1 drop=true name=audioappsink",
		}
		pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
		if err != nil {
			log.Error(ctx, "failed to create AI WHIP pipeline", "error", err)
			return
		}

		srcEle, err := pipeline.GetElementByName("aisrc")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP appsrc", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		src := app.SrcFromElement(srcEle)
		src.SetCallbacks(&app.SourceCallbacks{NeedDataFunc: ReaderNeedDataIncremental(ctx, pr)})

		videoEle, err := pipeline.GetElementByName("videoappsink")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP video appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		audioEle, err := pipeline.GetElementByName("audioappsink")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP audio appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}

		videoSink := app.SinkFromElement(videoEle)
		audioSink := app.SinkFromElement(audioEle)

		videoSink.SetCallbacks(mm.NewAISinkCallback(res.VideoCh))
		audioSink.SetCallbacks(mm.NewAISinkCallback(res.AudioCh))

		busErr := make(chan error, 1)
		go func() {
			busErr <- HandleBusMessages(ctx, pipeline)
		}()

		if err := pipeline.SetState(gst.StatePlaying); err != nil {
			log.Error(ctx, "failed to start AI WHIP pipeline", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}

		select {
		case <-ctx.Done():
			_ = pipeline.SetState(gst.StateNull)
			return
		case err := <-busErr:
			if err != nil && ctx.Err() == nil {
				log.Error(ctx, "AI WHIP pipeline error", "error", err)
			}
			_ = pipeline.SetState(gst.StateNull)
			return
		}
	}()

	return teedInput
}

// StartAISession sets up the AI gateway session for a stream.
// The input reader is consumed by a dedicated GStreamer pipeline that demuxes
// and forwards samples to the AI gateway via WHIP. This is used when input
// is a dedicated pipe (not shared with other consumers).
func (mm *MediaManager) StartAISession(
	ctx context.Context,
	input io.Reader,
	streamer string,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func(), bool) {
	resources, err := mm.SetupAISession(ctx, streamer, onTranscript)
	if err != nil {
		log.Log(ctx, "continuing without AI transcription", "reason", err)
		return input, func() {}, false
	}

	mm.startAIGatewayPipelineGeneric(ctx, input, resources)

	return input, resources.Cleanup, true
}

// startAIGatewayPipelineGeneric sets up a GStreamer pipeline for MP4/CMAF ingest.
// The input is consumed directly (not teed) as it's typically a dedicated pipe.
func (mm *MediaManager) startAIGatewayPipelineGeneric(
	ctx context.Context,
	input io.Reader,
	res *AISessionResources,
) {
	go func() {
		pipelineSlice := []string{
			"appsrc name=aisrc is-live=true format=time do-timestamp=true ! queue ! qtdemux name=demux",
			"demux. ! queue ! h264parse config-interval=-1 ! video/x-h264,stream-format=byte-stream ! appsink sync=false max-buffers=1 drop=true name=videoappsink",
			"demux. ! queue ! opusparse ! appsink sync=false max-buffers=1 drop=true name=audioappsink",
		}
		pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
		if err != nil {
			log.Error(ctx, "failed to create AI WHIP pipeline", "error", err)
			return
		}

		srcEle, err := pipeline.GetElementByName("aisrc")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP appsrc", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		src := app.SrcFromElement(srcEle)
		src.SetCallbacks(&app.SourceCallbacks{NeedDataFunc: ReaderNeedDataIncremental(ctx, input)})

		videoEle, err := pipeline.GetElementByName("videoappsink")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP video appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		audioEle, err := pipeline.GetElementByName("audioappsink")
		if err != nil {
			log.Error(ctx, "failed to get AI WHIP audio appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}

		videoSink := app.SinkFromElement(videoEle)
		audioSink := app.SinkFromElement(audioEle)

		videoSink.SetCallbacks(mm.NewAISinkCallback(res.VideoCh))
		audioSink.SetCallbacks(mm.NewAISinkCallback(res.AudioCh))

		busErr := make(chan error, 1)
		go func() {
			busErr <- HandleBusMessages(ctx, pipeline)
		}()

		if err := pipeline.SetState(gst.StatePlaying); err != nil {
			log.Error(ctx, "failed to start AI WHIP pipeline", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}

		select {
		case <-ctx.Done():
			_ = pipeline.SetState(gst.StateNull)
			return
		case err := <-busErr:
			if err != nil && ctx.Err() == nil {
				log.Error(ctx, "AI WHIP pipeline error", "error", err)
			}
			_ = pipeline.SetState(gst.StateNull)
			return
		}
	}()
}

// startTranscriptSSE starts a goroutine to read transcript events from the AI gateway SSE endpoint.
func (mm *MediaManager) startTranscriptSSE(
	ctx context.Context,
	dataStreamURL string,
	streamer string,
	logger client.Logger,
	onTranscript func(context.Context, client.TranscriptEvent),
) {
	go func() {
		if dataStreamURL == "" {
			log.Warn(ctx, "no data_url returned from AI gateway, skipping SSE")
			return
		}
		err := client.StreamTranscriptEvents(ctx, dataStreamURL, func(ctx context.Context, event client.TranscriptEvent) {
			log.Debug(ctx, "received transcript event",
				"type", event.Type,
				"timestamp_utc", event.TimestampUTC,
				"segments", len(event.Segments),
			)
			if onTranscript != nil {
				onTranscript(ctx, event)
			}
		}, logger)
		if err != nil && ctx.Err() == nil {
			log.Error(ctx, "SSE reader error", "error", err)
		}
	}()
}
