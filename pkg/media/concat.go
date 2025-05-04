package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

type ConcatStreamer interface {
	SubscribeSegment(ctx context.Context, user string, rendition string) <-chan *segchanman.Seg
	UnsubscribeSegment(ctx context.Context, user string, rendition string, ch <-chan *segchanman.Seg)
}

// This function remains in scope for the duration of a single users' playback
func ConcatStream(ctx context.Context, pipeline *gst.Pipeline, user string, rendition string, streamer ConcatStreamer) (*gst.Element, <-chan struct{}, error) {
	ctx = log.WithLogValues(ctx, "func", "ConcatStream")
	ctx, cancel := context.WithCancel(ctx)

	// make 1000000000000 elements!

	// input multiqueue
	inputQueue, err := gst.NewElementWithProperties("multiqueue", map[string]any{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create multiqueue element: %w", err)
	}
	err = pipeline.Add(inputQueue)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add input multiqueue to pipeline: %w", err)
	}
	inputQueuePadVideoSink := inputQueue.GetRequestPad("sink_%u")
	if inputQueuePadVideoSink == nil {
		return nil, nil, fmt.Errorf("failed to get input queue video sink pad")
	}
	inputQueuePadAudioSink := inputQueue.GetRequestPad("sink_%u")
	if inputQueuePadAudioSink == nil {
		return nil, nil, fmt.Errorf("failed to get input queue audio sink pad")
	}
	inputQueuePadVideoSrc := inputQueue.GetStaticPad("src_0")
	if inputQueuePadVideoSrc == nil {
		return nil, nil, fmt.Errorf("failed to get input queue video src pad")
	}
	inputQueuePadAudioSrc := inputQueue.GetStaticPad("src_1")
	if inputQueuePadAudioSrc == nil {
		return nil, nil, fmt.Errorf("failed to get input queue audio src pad")
	}
	// streamsynchronizer
	streamsynchronizer, err := gst.NewElementWithProperties("streamsynchronizer", map[string]any{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create streamsynchronizer element: %w", err)
	}

	err = pipeline.Add(streamsynchronizer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add streamsynchronizer to pipeline: %w", err)
	}
	syncPadVideoSink := streamsynchronizer.GetRequestPad("sink_%u")
	if syncPadVideoSink == nil {
		return nil, nil, fmt.Errorf("failed to get sync video sink pad")
	}
	syncPadAudioSink := streamsynchronizer.GetRequestPad("sink_%u")
	if syncPadAudioSink == nil {
		return nil, nil, fmt.Errorf("failed to get sync audio sink pad")
	}
	syncPadVideoSrc := streamsynchronizer.GetStaticPad("src_0")
	if syncPadVideoSrc == nil {
		return nil, nil, fmt.Errorf("failed to get sync video src pad")
	}
	syncPadAudioSrc := streamsynchronizer.GetStaticPad("src_1")
	if syncPadAudioSrc == nil {
		return nil, nil, fmt.Errorf("failed to get sync audio src pad")
	}

	// output multiqueue
	outputQueue, err := gst.NewElementWithProperties("multiqueue", map[string]any{
		"name": "concat-output-queue",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create multiqueue element: %w", err)
	}
	err = pipeline.Add(outputQueue)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add output multiqueue to pipeline: %w", err)
	}
	outputQueuePadVideoSink := outputQueue.GetRequestPad("sink_%u")
	if outputQueuePadVideoSink == nil {
		return nil, nil, fmt.Errorf("failed to get output queue video sink pad")
	}
	outputQueuePadAudioSink := outputQueue.GetRequestPad("sink_%u")
	if outputQueuePadAudioSink == nil {
		return nil, nil, fmt.Errorf("failed to get output queue audio sink pad")
	}
	// linking

	// input queue to streamsynchronizer
	ret := inputQueuePadVideoSrc.Link(syncPadVideoSink)
	if ret != gst.PadLinkOK {
		return nil, nil, fmt.Errorf("failed to link multiqueue to streamsynchronizer: %v", ret)
	}
	ret = inputQueuePadAudioSrc.Link(syncPadAudioSink)
	if ret != gst.PadLinkOK {
		return nil, nil, fmt.Errorf("failed to link multiqueue to streamsynchronizer: %v", ret)
	}

	// streamsynchronizer to output queue
	ret = syncPadVideoSrc.Link(outputQueuePadVideoSink)
	if ret != gst.PadLinkOK {
		return nil, nil, fmt.Errorf("failed to link streamsynchronizer to output queue: %v", ret)
	}
	ret = syncPadAudioSrc.Link(outputQueuePadAudioSink)
	if ret != gst.PadLinkOK {
		return nil, nil, fmt.Errorf("failed to link streamsynchronizer to output queue: %v", ret)
	}

	// ok now we can start looping over input files

	// this goroutine will read all the files from the segment queue and buffer
	// them in a pipe so that we don't miss any in between iterations of the output
	allFiles := make(chan []byte, 1024)
	go func() {
		ch := streamer.SubscribeSegment(ctx, user, rendition)
		defer streamer.UnsubscribeSegment(ctx, user, rendition, ch)
		for {
			select {
			case <-ctx.Done():
				log.Debug(ctx, "exiting segment reader")
				return
			case file := <-ch:
				log.Debug(ctx, "got segment", "file", file.Filepath)
				allFiles <- file.Data
				if len(file.Data) == 0 {
					log.Warn(ctx, "no more segments, stopping segment reader")
					return
				}
			}
		}
	}()

	segCount := 0

	// nextFile is the primary loop that pops off a file, creates new demuxer elements for it,
	// and pushes into the pipeline
	var nextFile func()
	nextFile = func() {
		mySegCount := segCount
		segCount += 1
		segDone := make(chan struct{})
		log.Debug(ctx, "moving to next file", "segCount", mySegCount)
		pr, pw := io.Pipe()
		go func() {
			select {
			case <-ctx.Done():
				pr.Close()
				pw.Close()
				return
			case bs := <-allFiles:
				if len(bs) == 0 {
					log.Warn(ctx, "no more segments, ending stream")
					pr.Close()
					pw.Close()
					cancel()
					return
				}
				_, err = io.Copy(pw, bytes.NewReader(bs))
				if err != nil {
					log.Error(ctx, "failed to copy segment file", "error", err)
					cancel()
					return
				}
				return
			}
		}()

		demux, err := gst.NewElementWithProperties("qtdemux", map[string]any{
			"name": fmt.Sprintf("concat-demux-%d", mySegCount),
		})
		if err != nil {
			log.Error(ctx, "failed to create demux element", "error", err)
			cancel()
			return
		}

		err = pipeline.Add(demux)
		if err != nil {
			log.Error(ctx, "failed to add demux to pipeline", "error", err)
			cancel()
			return
		}

		demuxSinkPad := demux.GetStaticPad("sink")
		if demuxSinkPad == nil {
			log.Error(ctx, "failed to get demux sink pad")
			cancel()
			return
		}

		mu := sync.Mutex{}
		count := 0
		_, err = demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
			mu.Lock()
			count += 1
			mu.Unlock()
			log.Debug(ctx, "demux pad-added", "name", pad.GetName(), "direction", pad.GetDirection())
			var downstreamPad *gst.Pad
			if strings.HasPrefix(pad.GetName(), "video_") {
				downstreamPad = inputQueuePadVideoSink
			} else if strings.HasPrefix(pad.GetName(), "audio_") {
				downstreamPad = inputQueuePadAudioSink
			} else {
				log.Error(ctx, "unknown pad", "name", pad.GetName(), "direction", pad.GetDirection())
				cancel()
				return
			}
			ret := pad.Link(downstreamPad)
			if ret != gst.PadLinkOK {
				log.Error(ctx, "failed to link demux to downstream pad", "name", pad.GetName(), "direction", pad.GetDirection(), "error", ret)
				cancel()
				return
			}
			if pad.GetDirection() == gst.PadDirectionSource {
				pad.AddProbe(gst.PadProbeTypeEventBoth, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
					if info.GetEvent().Type() != gst.EventTypeEOS {
						return gst.PadProbeOK
					}
					log.Debug(ctx, "demux EOS", "name", pad.GetName(), "direction", pad.GetDirection())
					pad.Unlink(downstreamPad)
					mu.Lock()
					defer mu.Unlock()
					count -= 1

					if count == 0 {
						// don't keep going if our context is done
						if ctx.Err() == nil {
							go nextFile()
							segDone <- struct{}{}
						}
					} else {
						log.Debug(ctx, "demux has more pads, waiting for them to close")
					}
					return gst.PadProbeRemove
				})
			}
		})
		if err != nil {
			log.Error(ctx, "failed to connect demux pad-added", "error", err)
			cancel()
			return
		}

		appsrc, err := gst.NewElementWithProperties("appsrc", map[string]any{
			"name":    fmt.Sprintf("concat-appsrc-%d", mySegCount),
			"is-live": true,
		})
		if err != nil {
			log.Error(ctx, "failed to get appsrc element from pipeline", "error", err)
			cancel()
			return
		}

		src := app.SrcFromElement(appsrc)

		appSrcPad := appsrc.GetStaticPad("src")
		if appSrcPad == nil {
			log.Error(ctx, "failed to get appsrc pad")
			cancel()
			return
		}

		done := func() {
			// appsrc.Unlink(demux)
			pads, err := src.GetPads()
			if err != nil {
				log.Error(ctx, "failed to get pads", "error", err)
				cancel()
				return
			}
			for _, pad := range pads {
				log.Debug(ctx, "setting pad-idle", "name", pad.GetName(), "direction", pad.GetDirection())

				pad.AddProbe(gst.PadProbeTypeIdle, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
					log.Debug(ctx, "pad-idle", "name", pad.GetName(), "direction", pad.GetDirection())
					src.EndStream()
					return gst.PadProbeRemove
				})
			}
		}

		src.SetAutomaticEOS(false)
		src.SetCallbacks(&app.SourceCallbacks{
			NeedDataFunc: func(self *app.Source, length uint) {
				bs := make([]byte, length)
				read, err := pr.Read(bs)
				if err != nil {
					if errors.Is(err, io.EOF) {
						if read > 0 {
							log.Debug(ctx, "got data on eof???")
							cancel()
							return
						}
						log.Debug(ctx, "EOF, ending segment", "length", read)
						done()
						return
					} else {
						log.Debug(ctx, "failed to read data, ending stream", "error", err)
						cancel()
						return
					}
				}
				toPush := bs
				if uint(read) < length {
					toPush = bs[:read]
				}
				buffer := gst.NewBufferWithSize(int64(len(toPush)))
				buffer.Map(gst.MapWrite).WriteData(toPush)
				defer buffer.Unmap()
				self.PushBuffer(buffer)

				if uint(read) < length {
					log.Debug(ctx, "short write, ending segment", "length", read)
					done()
				}
			},
		})
		err = pipeline.Add(appsrc)
		if err != nil {
			log.Error(ctx, "failed to add appsrc to pipeline", "error", err)
			cancel()
			return
		}

		ret := appSrcPad.Link(demuxSinkPad)
		if ret != gst.PadLinkOK {
			log.Error(ctx, "failed to link appsrc to demux", "error", ret)
			cancel()
			return
		}

		err = demux.SetState(gst.StatePlaying)
		if err != nil {
			log.Error(ctx, "failed to set demux state", "error", err)
			cancel()
			return
		}
		err = appsrc.SetState(gst.StatePlaying)
		if err != nil {
			log.Error(ctx, "failed to set appsrc state", "error", err)
			cancel()
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-segDone:
		}

		log.Debug(ctx, "ending segment")
		demux.SetState(gst.StateNull)
		src.SetCallbacks(&app.SourceCallbacks{})
		appsrc.SetState(gst.StateNull)
		pipeline.Remove(demux)
		pipeline.Remove(appsrc)
		pr.Close()
		pw.Close()
	}

	// fire it up!
	go nextFile()

	return outputQueue, ctx.Done(), nil
}
