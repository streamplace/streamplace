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

type AISessionResources struct {
	Session *client.TranscriptionSession
	VideoCh chan client.MediaSample
	AudioCh chan client.MediaSample
	Cleanup func()
}

func (mm *MediaManager) SetupAISession(ctx context.Context, streamer string, onTranscript func(context.Context, client.TranscriptEvent)) (*AISessionResources, error) {
	if mm.cli.AIGatewayBaseURL == "" {
		return nil, fmt.Errorf("AI gateway not configured")
	}

	streamName := fmt.Sprintf("streamplace-%s-%d", streamer, time.Now().UnixMilli())

	// wrap handler with transcript storage
	handler := func(ctx context.Context, event client.TranscriptEvent) {
		mm.transcriptStore.AddEvent(ctx, streamer, event)
		if onTranscript != nil {
			onTranscript(ctx, event)
		}
	}

	enableVideoIngress := true
	enableVideoEgress := true
	enableDataOutput := true
	cfg := client.StreamConfig{
		BaseURL:            mm.cli.AIGatewayBaseURL,
		Pipeline:           mm.cli.AIGatewayPipeline,
		EnableVideoIngress: &enableVideoIngress,
		EnableVideoEgress:  &enableVideoEgress,
		EnableDataOutput:   &enableDataOutput,
	}

	session := client.NewTranscriptionSession(ctx, cfg, streamName,
		client.WithLogger(newAIGatewayLogger()),
		client.WithTranscriptHandler(handler),
	)
	if err := session.Start(); err != nil {
		log.Error(ctx, "failed to start AI gateway session", "error", err)
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	log.Debug(ctx, "AI gateway session started", "streamName", streamName, "streamer", streamer)

	videoCh := make(chan client.MediaSample, 64)
	audioCh := make(chan client.MediaSample, 64)

	// forward samples from channels to WHIP
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-videoCh:
				if err := session.WriteVideoSample(s.Data, s.Duration); err != nil {
					log.Debug(ctx, "WHIP video write error", "error", err)
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
				if err := session.WriteAudioSample(s.Data, s.Duration); err != nil {
					log.Debug(ctx, "WHIP audio write error", "error", err)
				}
			}
		}
	}()

	cleanup := func() {
		if err := session.Stop(); err != nil {
			log.Error(ctx, "failed to stop AI gateway session", "error", err)
		}
		mm.transcriptStore.Clear(streamer)
	}

	return &AISessionResources{
		Session: session,
		VideoCh: videoCh,
		AudioCh: audioCh,
		Cleanup: cleanup,
	}, nil
}

func (mm *MediaManager) NewAISinkCallback(ch chan<- client.MediaSample) *app.SinkCallbacks {
	return &app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
			case ch <- client.MediaSample{Data: cpy, Duration: dur}:
			default:
			}
			return gst.FlowOK
		},
	}
}

func (mm *MediaManager) PublishTranscriptToBus(ctx context.Context, streamer string, event client.TranscriptEvent) {
	if mm.bus == nil {
		return
	}
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

func (mm *MediaManager) StartAISessionFromMKV(ctx context.Context, input io.Reader, streamer string, onTranscript func(context.Context, client.TranscriptEvent)) (io.Reader, func(), bool) {
	resources, err := mm.SetupAISession(ctx, streamer, onTranscript)
	if err != nil {
		log.Log(ctx, "continuing without AI transcription", "reason", err)
		return input, func() {}, false
	}

	// tee input to AI pipeline
	pr, pw := io.Pipe()
	asyncWriter := client.NewAsyncWriter(ctx, pw, newAIGatewayLogger())
	teedInput := io.TeeReader(input, asyncWriter)

	originalCleanup := resources.Cleanup
	cleanup := func() {
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
			log.Error(ctx, "failed to get appsrc element", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		src := app.SrcFromElement(srcEle)
		src.SetCallbacks(&app.SourceCallbacks{NeedDataFunc: ReaderNeedDataIncremental(ctx, pr)})

		videoEle, err := pipeline.GetElementByName("videoappsink")
		if err != nil {
			log.Error(ctx, "failed to get video appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}
		audioEle, err := pipeline.GetElementByName("audioappsink")
		if err != nil {
			log.Error(ctx, "failed to get audio appsink", "error", err)
			_ = pipeline.SetState(gst.StateNull)
			return
		}

		videoSink := app.SinkFromElement(videoEle)
		audioSink := app.SinkFromElement(audioEle)
		videoSink.SetCallbacks(mm.NewAISinkCallback(resources.VideoCh))
		audioSink.SetCallbacks(mm.NewAISinkCallback(resources.AudioCh))

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
		case err := <-busErr:
			if err != nil && ctx.Err() == nil {
				log.Error(ctx, "AI WHIP pipeline error", "error", err)
			}
			_ = pipeline.SetState(gst.StateNull)
		}
	}()

	return teedInput, cleanup, true
}
