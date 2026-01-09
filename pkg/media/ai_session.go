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

// StartAISessionFromMKV mirrors the legacy RTMP ingest tee: it demuxes MKV
// (H264+AAC) and forwards samples to the AI gateway via WHIP while streaming
// transcript events through the provided callback.
func (mm *MediaManager) StartAISessionFromMKV(
	ctx context.Context,
	input io.Reader,
	streamer string,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func(), bool) {
	if mm.cli.AIGatewayBaseURL == "" {
		return input, func() {}, false
	}

	cfg := mm.aiGatewayStreamConfig()
	logger := newAIGatewayLogger()

	streamName := fmt.Sprintf("streamplace-%s-%d", streamer, time.Now().UnixMilli())

	session, err := client.StartSession(ctx, cfg, streamName)
	if err != nil {
		log.Error(ctx, "failed to start AI gateway session, continuing without transcription", "error", err)
		return input, func() {}, false
	}

	log.Debug(ctx, "AI gateway session started",
		"streamID", session.ID,
		"dataURL", session.DataStreamURL,
		"streamer", streamer,
	)

	if session.WHIPURL == "" {
		log.Error(ctx, "AI gateway did not provide whip_url, continuing without transcription", "streamID", session.ID)
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return input, func() {}, false
	}

	teedInput, cleanup := mm.startAIGatewayWHIPMKV(ctx, input, streamer, cfg, session, logger, onTranscript)

	return teedInput, cleanup, true
}

// startAIGatewayWHIPMKV sets up WHIP publishing for MKV (H264+AAC) ingest,
// mirroring the legacy RTMP path.
func (mm *MediaManager) startAIGatewayWHIPMKV(
	ctx context.Context,
	input io.Reader,
	streamer string,
	cfg client.StreamConfig,
	session *client.StreamSession,
	logger client.Logger,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func()) {
	pr, pw := io.Pipe()
	asyncWriter := client.NewAsyncWriter(ctx, pw, logger)
	teedInput := io.TeeReader(input, asyncWriter)

	publisher := client.NewWHIPPublisher(ctx, session.WHIPURL, logger)
	if err := publisher.Start(); err != nil {
		log.Error(ctx, "failed to start WHIP publisher, continuing without transcription", "error", err)
		_ = asyncWriter.Close()
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return input, func() {}
	}

	videoCh := make(chan struct {
		data []byte
		dur  time.Duration
	}, 64)
	audioCh := make(chan struct {
		data []byte
		dur  time.Duration
	}, 64)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-videoCh:
				if err := publisher.WriteVideoSample(s.data, s.dur); err != nil {
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
				if err := publisher.WriteAudioSample(s.data, s.dur); err != nil {
					log.Debug(ctx, "WHIP write audio sample error", "error", err)
				}
			}
		}
	}()

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

		videoSink.SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
			case videoCh <- struct {
				data []byte
				dur  time.Duration
			}{data: cpy, dur: dur}:
			default:
			}
			return gst.FlowOK
		}})

		audioSink.SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
			case audioCh <- struct {
				data []byte
				dur  time.Duration
			}{data: cpy, dur: dur}:
			default:
			}
			return gst.FlowOK
		}})

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

	mm.startTranscriptSSE(ctx, session.DataStreamURL, streamer, logger, onTranscript)

	cleanup := mm.makeAIGatewayCleanup(ctx, cfg, session.ID, streamer, asyncWriter, publisher.Stop)

	return teedInput, cleanup
}

// StartAISession sets up the AI gateway session for a stream and returns a reader
// that tees media into the gateway along with a cleanup function.
func (mm *MediaManager) StartAISession(
	ctx context.Context,
	input io.Reader,
	streamer string,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func(), bool) {
	if mm.cli.AIGatewayBaseURL == "" {
		return input, func() {}, false
	}

	cfg := mm.aiGatewayStreamConfig()
	logger := newAIGatewayLogger()

	streamName := fmt.Sprintf("streamplace-%s-%d", streamer, time.Now().UnixMilli())

	session, err := client.StartSession(ctx, cfg, streamName)
	if err != nil {
		log.Error(ctx, "failed to start AI gateway session, continuing without transcription", "error", err)
		return input, func() {}, false
	}

	log.Debug(ctx, "AI gateway session started",
		"streamID", session.ID,
		"dataURL", session.DataStreamURL,
		"streamer", streamer,
	)

	if session.WHIPURL == "" {
		log.Error(ctx, "AI gateway did not provide whip_url, continuing without transcription", "streamID", session.ID)
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return input, func() {}, false
	}

	teedInput, cleanup := mm.startAIGatewayWHIP(ctx, input, streamer, cfg, session, logger, onTranscript)

	return teedInput, cleanup, true
}

// startAIGatewayWHIP sets up WHIP-based media publishing to the AI gateway.
// It demuxes MP4 stream fragments and sends H.264/Opus samples over WebRTC.
func (mm *MediaManager) startAIGatewayWHIP(
	ctx context.Context,
	input io.Reader,
	streamer string,
	cfg client.StreamConfig,
	session *client.StreamSession,
	logger client.Logger,
	onTranscript func(context.Context, client.TranscriptEvent),
) (io.Reader, func()) {
	publisher := client.NewWHIPPublisher(ctx, session.WHIPURL, logger)
	if err := publisher.Start(); err != nil {
		log.Error(ctx, "failed to start WHIP publisher, continuing without transcription", "error", err)
		_ = client.StopSession(context.Background(), cfg, session.ID)
		return input, func() {}
	}

	videoCh := make(chan struct {
		data []byte
		dur  time.Duration
	}, 64)
	audioCh := make(chan struct {
		data []byte
		dur  time.Duration
	}, 64)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-videoCh:
				if err := publisher.WriteVideoSample(s.data, s.dur); err != nil {
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
				if err := publisher.WriteAudioSample(s.data, s.dur); err != nil {
					log.Debug(ctx, "WHIP write audio sample error", "error", err)
				}
			}
		}
	}()

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

		videoSink.SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
			case videoCh <- struct {
				data []byte
				dur  time.Duration
			}{data: cpy, dur: dur}:
			default:
			}
			return gst.FlowOK
		}})

		audioSink.SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
			case audioCh <- struct {
				data []byte
				dur  time.Duration
			}{data: cpy, dur: dur}:
			default:
			}
			return gst.FlowOK
		}})

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

	mm.startTranscriptSSE(ctx, session.DataStreamURL, streamer, logger, onTranscript)

	cleanup := mm.makeAIGatewayCleanup(ctx, cfg, session.ID, streamer, nil, publisher.Stop)

	return input, cleanup
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
			mm.transcriptStore.AddEvent(ctx, streamer, event)
			if onTranscript != nil {
				onTranscript(ctx, event)
			}
		}, logger)
		if err != nil && ctx.Err() == nil {
			log.Error(ctx, "SSE reader error", "error", err)
		}
	}()
}

// makeAIGatewayCleanup creates a cleanup function for AI gateway resources.
func (mm *MediaManager) makeAIGatewayCleanup(
	ctx context.Context,
	cfg client.StreamConfig,
	sessionID string,
	streamer string,
	asyncWriter *client.AsyncWriter,
	stopPublisher func(),
) func() {
	return func() {
		if asyncWriter != nil {
			written, dropped := asyncWriter.Stats()
			log.Log(ctx, "AI gateway tee stats", "written", written, "dropped", dropped, "streamer", streamer)
			_ = asyncWriter.Close()
		}
		stopPublisher()

		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StopSession(stopCtx, cfg, sessionID); err != nil {
			log.Error(ctx, "failed to stop AI gateway session", "error", err)
		} else {
			log.Log(ctx, "AI gateway session stopped", "streamID", sessionID)
		}

		mm.transcriptStore.Clear(streamer)
	}
}
