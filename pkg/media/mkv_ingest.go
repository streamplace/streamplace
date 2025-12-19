package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/aigateway"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
)

// ingest a H264+AAC MKV stream (prolly from an RTMP server)
func (mm *MediaManager) MKVIngest(ctx context.Context, input io.Reader, ms MediaSigner) error {
	shouldRecord, err := mm.shouldRecord(ctx, ms.Streamer())
	if err != nil {
		return err
	}
	if shouldRecord {
		log.Log(ctx, "recording RTMP stream to file", "streamer", ms.Streamer())
		pr, pw := io.Pipe()
		input = io.TeeReader(input, pw)
		go func() {
			err := mm.dumpToFile(ctx, pr, ms.Streamer(), ".rtmp.mkv")
			if err != nil {
				log.Error(ctx, "error dumping to file", "error", err)
			}
		}()
	} else {
		log.Log(ctx, "not recording RTMP stream to file", "streamer", ms.Streamer())
	}

	var aiCleanup func()
	if mm.cli.AIGatewayBaseURL != "" {
		input, aiCleanup = mm.startAIGatewayTee(ctx, input, ms.Streamer())
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		if aiCleanup != nil {
			aiCleanup()
		}
	}()
	pipelineSlice := []string{
		"appsrc name=streamsrc ! matroskademux name=demux",
		"demux. ! queue ! h264parse name=parse",
		"demux. ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating MKVIngest pipeline: %w", err)
	}

	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return err
	}
	// defer runtime.KeepAlive(srcele)
	src := app.SrcFromElement(srcele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	parseEle, err := pipeline.GetElementByName("parse")
	if err != nil {
		return err
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return err
	}
	err = parseEle.Link(signer)
	if err != nil {
		return err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return err
	}
	err = audioenc.Link(signer)
	if err != nil {
		return err
	}

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		busErr <- err
	}()

	go mm.HandleKeyRevocation(ctx, ms, pipeline)

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	err = <-busErr

	return err
}

// startAIGatewayTee creates a tee that copies ingest data to the AI gateway for transcription.
// It returns a wrapped reader and a cleanup function that must be called when done.
// If the gateway connection fails, it logs the error and returns the original input unchanged.
func (mm *MediaManager) startAIGatewayTee(ctx context.Context, input io.Reader, streamer string) (io.Reader, func()) {
	cfg := aigateway.Config{
		BaseURL:       mm.cli.AIGatewayBaseURL,
		PathPrefix:    mm.cli.AIGatewayPathPrefix,
		RewriteURLsTo: mm.cli.AIGatewayRewriteURLsTo,
		Pipeline:      mm.cli.AIGatewayPipeline,
		RTMPHost:      mm.cli.AIGatewayRTMPHost,
	}

	streamName := fmt.Sprintf("streamplace-%s-%d", streamer, time.Now().UnixMilli())

	session, err := aigateway.StartStream(ctx, cfg, streamName)
	if err != nil {
		log.Error(ctx, "failed to start AI gateway session, continuing without transcription", "error", err)
		return input, func() {}
	}

	log.Log(ctx, "AI gateway session started",
		"streamID", session.ID,
		"dataURL", session.DataURL,
		"streamer", streamer,
	)

	if session.WhipURL != "" {
		return mm.startAIGatewayWHIP(ctx, input, streamer, cfg, session)
	}

	rtmpURL := session.RTMPURL
	if cfg.RTMPHost != "" {
		rtmpURL = session.ConstructRTMPURL(cfg.RTMPHost)
	}
	if rtmpURL == "" {
		log.Error(ctx, "AI gateway did not provide rtmp_url and no --ai-gateway-rtmp-host set, continuing without transcription", "streamID", session.ID)
		_ = aigateway.StopStream(context.Background(), cfg, session.ID)
		return input, func() {}
	}
	publisher := aigateway.NewRTMPPublisher(ctx, mm.cli.AIGatewayFFmpegBin, rtmpURL)

	stdin, err := publisher.Start()
	if err != nil {
		log.Error(ctx, "failed to start RTMP publisher, continuing without transcription", "error", err)
		_ = aigateway.StopStream(context.Background(), cfg, session.ID)
		return input, func() {}
	}

	asyncWriter := aigateway.NewAsyncWriter(ctx, stdin)
	teedInput := io.TeeReader(input, asyncWriter)

	mm.startSSEReader(ctx, session.DataURL, streamer)

	cleanup := mm.makeAIGatewayCleanup(ctx, cfg, session.ID, streamer, asyncWriter, publisher.Stop)

	return teedInput, cleanup
}

// startAIGatewayWHIP sets up WHIP-based media publishing to the AI gateway.
// It demuxes the MKV stream and sends H.264/Opus samples over WebRTC.
func (mm *MediaManager) startAIGatewayWHIP(ctx context.Context, input io.Reader, streamer string, cfg aigateway.Config, session *aigateway.Session) (io.Reader, func()) {
	pr, pw := io.Pipe()
	asyncWriter := aigateway.NewAsyncWriter(ctx, pw)
	teedInput := io.TeeReader(input, asyncWriter)

	publisher := aigateway.NewWHIPPublisher(ctx, session.WhipURL)
	if err := publisher.Start(); err != nil {
		log.Error(ctx, "failed to start WHIP publisher, continuing without transcription", "error", err)
		_ = asyncWriter.Close()
		_ = aigateway.StopStream(context.Background(), cfg, session.ID)
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
			"demux. ! queue ! fdkaacdec ! audioresample ! opusenc ! appsink sync=false max-buffers=1 drop=true name=audioappsink",
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

	mm.startSSEReader(ctx, session.DataURL, streamer)

	cleanup := mm.makeAIGatewayCleanup(ctx, cfg, session.ID, streamer, asyncWriter, publisher.Stop)

	return teedInput, cleanup
}

// startSSEReader starts a goroutine to read transcript events from the AI gateway SSE endpoint.
func (mm *MediaManager) startSSEReader(ctx context.Context, dataURL string, streamer string) {
	go func() {
		if dataURL == "" {
			log.Warn(ctx, "no data_url returned from AI gateway, skipping SSE")
			return
		}
		err := aigateway.ReadSSE(ctx, dataURL, func(ctx context.Context, event aigateway.TranscriptEvent) {
			log.Debug(ctx, "received transcript event",
				"type", event.Type,
				"text", event.Text,
				"timestamp_ms", event.TimestampMS,
			)
			mm.transcriptStore.AddEvent(streamer, event)
		})
		if err != nil && ctx.Err() == nil {
			log.Error(ctx, "SSE reader error", "error", err)
		}
	}()
}

// makeAIGatewayCleanup creates a cleanup function for AI gateway resources.
func (mm *MediaManager) makeAIGatewayCleanup(
	ctx context.Context,
	cfg aigateway.Config,
	sessionID string,
	streamer string,
	asyncWriter *aigateway.AsyncWriter,
	stopPublisher func(),
) func() {
	return func() {
		written, dropped := asyncWriter.Stats()
		log.Log(ctx, "AI gateway tee stats", "written", written, "dropped", dropped, "streamer", streamer)

		_ = asyncWriter.Close()
		stopPublisher()

		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := aigateway.StopStream(stopCtx, cfg, sessionID); err != nil {
			log.Error(ctx, "failed to stop AI gateway session", "error", err)
		} else {
			log.Log(ctx, "AI gateway session stopped", "streamID", sessionID)
		}

		mm.transcriptStore.Clear(streamer)
	}
}

func (mm *MediaManager) dumpToFile(ctx context.Context, r io.Reader, user string, filesuffix string) error {
	now := aqtime.FromTime(time.Now())
	filename := fmt.Sprintf("%s%s", now.FileSafeString(), filesuffix)
	f, err := mm.cli.DataFileCreate([]string{"debug-recordings", user, filename}, false)
	if err != nil {
		return fmt.Errorf("failed to create data file: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return fmt.Errorf("failed to copy to file: %w", err)
	}
	return nil
}
