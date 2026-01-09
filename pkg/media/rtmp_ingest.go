package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/muxionlabs/ai-go-sdk/pkg/client"
	"stream.place/streamplace/pkg/log"
)

type RTMPH264Data struct {
	AU  [][]byte
	PTS time.Duration
	DTS time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan   chan any
	VideoTrack  *format.H264
	AudioTrack  *format.MPEG4Audio
	MediaSigner MediaSigner
}

func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner) error {
	var aiCleanup func()
	var videoCh chan struct {
		data []byte
		dur  time.Duration
	}
	var audioCh chan struct {
		data []byte
		dur  time.Duration
	}

	if mm.cli.AIGatewayBaseURL != "" {
		onTranscript := func(ctx context.Context, event client.TranscriptEvent) {
			mm.transcriptStore.AddEvent(ctx, ms.Streamer(), event)
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
					mm.bus.Publish(ms.Streamer(), msg)
				}
			}
		}

		cfg := client.StreamConfig{
			BaseURL:  mm.cli.AIGatewayBaseURL,
			Pipeline: mm.cli.AIGatewayPipeline,
		}
		logger := newAIGatewayLogger()

		streamName := fmt.Sprintf("streamplace-%s-%d", ms.Streamer(), time.Now().UnixMilli())
		session, err := client.StartSession(ctx, cfg, streamName)
		if err != nil {
			log.Error(ctx, "failed to start AI gateway session, continuing without transcription", "error", err)
		} else {
			log.Debug(ctx, "AI gateway session started",
				"streamID", session.ID,
				"dataURL", session.DataStreamURL,
				"streamer", ms.Streamer(),
			)

			if session.WHIPURL == "" {
				log.Error(ctx, "AI gateway did not provide whip_url, continuing without transcription", "streamID", session.ID)
				_ = client.StopSession(context.Background(), cfg, session.ID)
			} else {
				aiCtx, aiCancel := context.WithCancel(context.Background())
				mm.startTranscriptSSE(aiCtx, session.DataStreamURL, ms.Streamer(), logger, onTranscript)

				publisher := client.NewWHIPPublisher(ctx, session.WHIPURL, logger)
				if err := publisher.Start(); err != nil {
					log.Error(ctx, "failed to start WHIP publisher, continuing without transcription", "error", err)
					aiCancel()
					_ = client.StopSession(context.Background(), cfg, session.ID)
				} else {
					videoCh = make(chan struct {
						data []byte
						dur  time.Duration
					}, 1024)
					audioCh = make(chan struct {
						data []byte
						dur  time.Duration
					}, 1024)

					go func() {
						for {
							select {
							case <-aiCtx.Done():
								return
							case s := <-videoCh:
								if err := publisher.WriteVideoSample(s.data, s.dur); err != nil {
									log.Debug(aiCtx, "WHIP write video sample error", "error", err)
								}
							}
						}
					}()
					go func() {
						for {
							select {
							case <-aiCtx.Done():
								return
							case s := <-audioCh:
								if err := publisher.WriteAudioSample(s.data, s.dur); err != nil {
									log.Debug(aiCtx, "WHIP write audio sample error", "error", err)
								}
							}
						}
					}()

					aiCleanup = func() {
						aiCancel()
						publisher.Stop()
						stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := client.StopSession(stopCtx, cfg, session.ID); err != nil {
							log.Error(ctx, "failed to stop AI gateway session", "error", err)
						}
						mm.transcriptStore.Clear(ms.Streamer())
					}
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		if aiCleanup != nil {
			aiCleanup()
		}
	}()

	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
	}

	if aiCleanup != nil {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc ! tee name=audiotee",
			"audiotee. ! queue ! appsink name=ai_audio_sink sync=false drop=true max-buffers=1",
			"audiotee. ! queue name=audioenc",
			"demux.video ! queue ! tee name=videotee",
			"videotee. ! queue ! h264parse config-interval=-1 ! video/x-h264,stream-format=byte-stream,alignment=au ! appsink name=ai_video_sink sync=false drop=true max-buffers=10",
			"videotee. ! queue ! h264parse name=parse",
		)
	} else {
		pipelineSlice = append(pipelineSlice,
			"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
			"demux.video ! queue ! h264parse name=parse",
		)
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	if aiCleanup != nil {
		videoEle, err := pipeline.GetElementByName("ai_video_sink")
		if err != nil {
			return fmt.Errorf("failed to get AI video appsink: %w", err)
		}
		audioEle, err := pipeline.GetElementByName("ai_audio_sink")
		if err != nil {
			return fmt.Errorf("failed to get AI audio appsink: %w", err)
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
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}

	parseEle, err := pipeline.GetElementByName("parse")
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
