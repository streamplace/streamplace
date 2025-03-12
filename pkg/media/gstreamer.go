package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/test"
)

const HLS_PLAYLIST = "stream.m3u8"

// Pipe with a mechanism to keep the FDs not garbage collected
func SafePipe() (*os.File, *os.File, func(), error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}
	return r, w, func() {
		runtime.KeepAlive(r.Fd())
		runtime.KeepAlive(w.Fd())
	}, nil
}

func ReaderNeedData(ctx context.Context, input io.Reader) func(self *app.Source, length uint) {
	return func(self *app.Source, length uint) {
		if ctx.Err() != nil {
			self.EndStream()
			return
		}
		bs := make([]byte, length)
		read, err := input.Read(bs)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if read > 0 {
					panic("got data on eof???")
				}
				log.Debug(ctx, "EOF, ending stream", "length", read)
				self.EndStream()
				return
			} else {
				panic(err)
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
	}
}

func WriterNewSample(ctx context.Context, output io.Writer) func(sink *app.Sink) gst.FlowReturn {
	return func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowOK
		}

		// Retrieve the buffer from the sample.
		buffer := sample.GetBuffer()

		_, err := io.Copy(output, buffer.Reader())

		if err != nil {
			panic(err)
		}

		return gst.FlowOK
	}
}

func AddOpusToMKV(ctx context.Context, input io.Reader, output io.Writer) error {
	pipelineSlice := []string{
		"appsrc name=appsrc ! matroskademux name=demux",
		"matroskamux name=mux ! appsink name=appsink",
		"demux.audio_0 ! queue ! tee name=asplit",
		"demux.video_0 ! queue ! mux.video_0",
		"asplit. ! queue ! fdkaacdec ! audioresample ! opusenc inband-fec=true perfect-timestamp=true bitrate=128000 ! mux.audio_1",
		"asplit. ! queue ! mux.audio_0",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return err
	}

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, output),
		EOSFunc: func(sink *app.Sink) {
			cancel()
		},
	})

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)
	return nil
}

// basic test to make sure gstreamer functionality is working
func SelfTest(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	f, err := test.Files.Open("fixtures/sample-segment.mp4")
	if err != nil {
		return err
	}
	defer f.Close()
	bs, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	pipeline, err := gst.NewPipelineFromString("appsrc name=src ! appsink name=sink")
	if err != nil {
		return err
	}

	srcele, err := pipeline.GetElementByName("src")
	if err != nil {
		return err
	}
	if srcele == nil {
		return fmt.Errorf("srcele not found")
	}
	src := app.SrcFromElement(srcele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, _ uint) {
			buffer := gst.NewBufferWithSize(int64(len(bs)))
			buffer.Map(gst.MapWrite).WriteData(bs)
			defer buffer.Unmap()
			self.PushBuffer(buffer)
			self.EndStream()
		},
	})

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	output := &bytes.Buffer{}
	sinkele, err := pipeline.GetElementByName("sink")
	if err != nil {
		return err
	}
	if sinkele == nil {
		return fmt.Errorf("sinkele not found")
	}
	appsink := app.SinkFromElement(sinkele)
	appsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowOK
			}
			// defer sample.Unref()

			// Retrieve the buffer from the sample.
			buffer := sample.GetBuffer()

			_, err := io.Copy(output, buffer.Reader())

			if err != nil {
				panic(err)
			}

			return gst.FlowOK
		},
		EOSFunc: func(sink *app.Sink) {
			cancel()
		},
	})

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	go func() {
		<-ctx.Done()
		mainLoop.Quit()
	}()

	mainLoop.Run()

	if len(output.Bytes()) < 1 {
		return fmt.Errorf("got a zero-byte buffer from SelfTest")
	}
	return nil
}

// #EXTM3U
// #EXT-X-VERSION:3
// #EXT-X-MEDIA-SEQUENCE:281
// #EXT-X-TARGETDURATION:1

// #EXTINF:1,
// segment00281.ts
// #EXTINF:1.0049999952316284,
// segment00282.ts
// #EXTINF:1,
// segment00283.ts
// #EXTINF:1.0010000467300415,
// segment00284.ts
// #EXTINF:1,
// segment00285.ts
// #EXT-X-ENDLIST

func (mm *MediaManager) ToHLS(ctx context.Context, input io.Reader, m3u8 *M3U8) error {
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ToHLS")

	splitmuxsink, err := gst.NewElementWithProperties("splitmuxsink", map[string]any{
		"name":           "mux",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mpegtsmux",
		"max-size-bytes": 1,
	})
	if err != nil {
		return err
	}

	p := splitmuxsink.GetRequestPad("video")
	if p == nil {
		return fmt.Errorf("failed to get video pad")
	}
	p = splitmuxsink.GetRequestPad("audio_%u")
	if p == nil {
		return fmt.Errorf("failed to get audio pad")
	}

	pipelineSlice := []string{
		"appsrc name=appsrc ! matroskademux name=demux",
		"demux.video_0 ! queue ! h264parse name=videoparse",
		"demux.audio_0 ! queue ! opusdec use-inband-fec=true ! audioresample ! fdkaacenc name=audioenc",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating ToHLS pipeline: %w", err)
	}

	err = pipeline.Add(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error adding splitmuxsink to ToHLS pipeline: %w", err)
	}

	videoparse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("error getting videoparse from ToHLS pipeline: %w", err)
	}
	err = videoparse.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking videoparse to splitmuxsink: %w", err)
	}

	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return fmt.Errorf("error getting audioenc from ToHLS pipeline: %w", err)
	}
	err = audioenc.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking audioenc to splitmuxsink: %w", err)
	}

	splitmuxsink.Connect("sink-added", func(split, sinkEle *gst.Element) {
		vf, err := m3u8.GetNextSegment(ctx)
		if err != nil {
			panic(err)
		}
		appsink := app.SinkFromElement(sinkEle)
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, vf.Buf),
			EOSFunc: func(sink *app.Sink) {
				m3u8.CloseSegment(ctx, vf)
			},
		})
	})

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
	})

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		caps := pad.GetCurrentCaps()
		if caps == nil {
			fmt.Println("Unable to get pad caps")
			return
		}

		fmt.Printf("New pad added: %s\n", pad.GetName())
		fmt.Printf("Caps: %s\n", caps.String())

		structure := caps.GetStructureAt(0)
		if structure == nil {
			fmt.Println("Unable to get structure from caps")
			return
		}

		name := structure.Name()
		fmt.Printf("Structure Name: %s\n", name)

		if name[:5] == "video" {
			// Get some common video properties
			widthVal, _ := structure.GetValue("width")
			heightVal, _ := structure.GetValue("height")

			width, ok := widthVal.(int)
			if ok {
				m3u8.Width = uint64(width)
			}
			height, ok := heightVal.(int)
			if ok {
				m3u8.Height = uint64(height)
			}
			// framerate, ok := framerateVal.(string)
			// if ok {
			// 	fmt.Printf("  Framerate: %s\n", framerate)
			// }
			// pixelAspectRatio, ok := pixelAspectRatioVal.(string)
			// if ok {
			// 	fmt.Printf("  Pixel Aspect Ratio: %s\n", pixelAspectRatio)
			// }
			// if codecVal != nil {
			// 	fmt.Printf("  Has codec data: true\n")
			// }
		}

		// if name[:5] == "audio" {
		// 	// Get some common audio properties
		// 	rateVal, _ := structure.GetValue("rate")
		// 	channelsVal, _ := structure.GetValue("channels")
		// 	formatVal, err := structure.GetValue("format")
		// 	mpegversion, _ := structure.GetValue("mpegversion")
		// 	log.Log(ctx, "format error", "error", err, "mpegversion", mpegversion)

		// 	fmt.Printf("  Structure: %s\n", structure.String())
		// 	rate, ok := rateVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Rate: %d\n", rate)
		// 	}
		// 	channels, ok := channelsVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Channels: %d\n", channels)
		// 	}
		// 	format, ok := formatVal.(int)
		// 	if ok {
		// 		fmt.Printf("  Format: %d\n", format)
		// 	}

		// }
	}

	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return err
	}
	demux.Connect("pad-added", onPadAdded)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		HandleBusMessagesCustom(ctx, pipeline, func(msg *gst.Message) {
			switch msg.Type() {
			case gst.MessageElement:
				structure := msg.GetStructure()
				name := structure.Name()
				if name == "splitmuxsink-fragment-opened" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Warn(ctx, "splitmuxsink-fragment-opened error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-opened not a uint64")
						cancel()
					}
					m3u8.FragmentOpened(ctx, runningTimeInt)
				}
				if name == "splitmuxsink-fragment-closed" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Warn(ctx, "splitmuxsink-fragment-closed error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-closed not a uint64")
						cancel()
					}
					m3u8.FragmentClosed(ctx, runningTimeInt)
				}
			}
		})
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}

func (mm *MediaManager) IngestStream(ctx context.Context, input io.Reader, ms MediaSigner) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=streamsrc ! matroskademux name=demux",
		"demux. ! queue ! h264parse name=parse",
		"demux. ! queue ! aacparse name=audioparse",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating IngestStream pipeline: %w", err)
	}
	defer runtime.KeepAlive(pipeline)
	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return err
	}
	// defer runtime.KeepAlive(srcele)
	src := app.SrcFromElement(srcele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, input),
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
	audioparse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return err
	}
	err = audioparse.Link(signer)
	if err != nil {
		return err
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

const TESTSRC_WIDTH = 1280
const TESTSRC_HEIGHT = 720
const QR_SIZE = 256

type QRData struct {
	Now int64 `json:"now"`
}

func (mm *MediaManager) TestSource(ctx context.Context, ms MediaSigner) error {
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipelineSlice := []string{
		"h264parse name=videoparse",
		"compositor name=comp ! videoconvert ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast key-int-max=30 ! queue ! videoparse.",
		fmt.Sprintf(`videotestsrc is-live=true ! video/x-raw,format=AYUV,framerate=30/1,width=%d,height=%d ! comp.`, TESTSRC_WIDTH, TESTSRC_HEIGHT),
		fmt.Sprintf("videobox border-alpha=0 top=-%d left=-%d name=box ! comp.", (TESTSRC_HEIGHT/2)-(QR_SIZE/2), (TESTSRC_WIDTH/2)-(QR_SIZE/2)),
		"appsrc name=pngsrc ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! box.",
		"appsrc name=timetext ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! comp.",
		"audiotestsrc ! audioconvert ! opusenc inband-fec=true perfect-timestamp=true bitrate=128000 ! queue ! opusparse name=audioparse",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating TestSource pipeline: %w", err)
	}

	videoparse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return err
	}

	audioparse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return err
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	pipeline.Add(signer)

	err = videoparse.Link(signer)
	if err != nil {
		return fmt.Errorf("link to signer failed: %w", err)
	}
	err = audioparse.Link(signer)
	if err != nil {
		return fmt.Errorf("link to signer failed: %w", err)
	}

	pngele, err := pipeline.GetElementByName("pngsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(pngele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, _ uint) {
			now := time.Now().UnixMilli()
			data := QRData{Now: now}
			bs, err := json.Marshal(data)
			if err != nil {
				panic(err)
			}
			png, err := qrcode.Encode(string(bs), qrcode.Medium, 256)
			if err != nil {
				panic(err)
			}
			buffer := gst.NewBufferWithSize(int64(len(png)))
			buffer.Map(gst.MapWrite).WriteData(png)
			defer buffer.Unmap()
			self.PushBuffer(buffer)
		},
	})
	tr, err := NewTextRenderer()
	if err != nil {
		return err
	}
	timetext, err := pipeline.GetElementByName("timetext")
	if err != nil {
		return err
	}

	timesrc := app.SrcFromElement(timetext)
	timesrc.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, _ uint) {
			aqt := aqtime.FromTime(time.Now())
			png, err := tr.GenerateImage(aqt.String(), "#ffffff", "#000000", 36)
			if err != nil {
				panic(err)
			}
			buffer := gst.NewBufferWithSize(int64(len(png)))
			buffer.Map(gst.MapWrite).WriteData(png)
			defer buffer.Unmap()
			self.PushBuffer(buffer)
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
		mainLoop.Quit()
	}()

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		mainLoop.Run()
		log.Log(ctx, "main loop complete")
		return nil
	})

	return g.Wait()
}

// element that takes the input stream, muxes to mp4, and signs the result
func (mm *MediaManager) SegmentAndSignElem(ctx context.Context, ms MediaSigner) (*gst.Element, error) {
	// elem, err := gst.NewElement("splitmuxsink name=splitter async-finalize=true sink-factory=appsink muxer-factory=matroskamux max-size-bytes=1")
	elem, err := gst.NewElementWithProperties("splitmuxsink", map[string]any{
		"name":           "signer",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mp4mux",
		"max-size-bytes": 1,
	})
	if err != nil {
		return nil, err
	}

	p := elem.GetRequestPad("video")
	if p == nil {
		return nil, fmt.Errorf("failed to get video pad")
	}
	p = elem.GetRequestPad("audio_%u")
	if p == nil {
		return nil, fmt.Errorf("failed to get audio pad")
	}

	resetTimer := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-resetTimer:
				continue
			case <-time.After(time.Second * 10):
				log.Warn(ctx, "no new segment for 10 seconds")
				elem.ErrorMessage(gst.DomainCore, gst.CoreErrorFailed, "No new segment for 10 seconds", "No new segment for 10 seconds (debug)")
				return
			}
		}
	}()

	elem.Connect("sink-added", func(split, sinkEle *gst.Element) {
		buf := &bytes.Buffer{}
		appsink := app.SinkFromElement(sinkEle)
		if appsink == nil {
			panic("appsink should not be nil")
		}
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, buf),
			EOSFunc: func(sink *app.Sink) {
				resetTimer <- struct{}{}
				bs, err := ms.SignMP4(ctx, bytes.NewReader(buf.Bytes()), time.Now().UnixMilli())
				if err != nil {
					log.Error(ctx, "error signing segment", "error", err)
					return
				}
				err = mm.ValidateMP4(ctx, bytes.NewReader(bs))
				if err != nil {
					log.Error(ctx, "error validating segment", "error", err)
					return
				}
			},
		})
	})

	return elem, nil
}

func (mm *MediaManager) Thumbnail(ctx context.Context, r io.Reader, w io.Writer) error {
	ctx = log.WithLogValues(ctx, "function", "Thumbnail")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux ! decodebin ! videoconvert ! videoscale ! video/x-raw,width=[1,720],height=[1,720],pixel-aspect-ratio=1/1 ! pngenc snapshot=true ! appsink name=appsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating Thumbnail pipeline: %w", err)
	}
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, r),
	})

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			cancel()
		},
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}

func (mm *MediaManager) MP4Playback(ctx context.Context, user string, w io.Writer) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	ctx = log.WithLogValues(ctx, "playbackID", uu.String())
	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "mediafunc", "MP4Playback")

	pipelineSlice := []string{
		"mp4mux name=muxer fragment-mode=first-moov-then-finalise fragment-duration=1000 streamable=true ! appsink name=mp4sink",
		"h264parse name=videoparse ! muxer.",
		"opusparse name=audioparse ! muxer.",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	outputQueue, done, err := ConcatStream(ctx, pipeline, user, mm)
	if err != nil {
		return fmt.Errorf("failed to get output queue: %w", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	err = outputQueue.Link(videoParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to video parse: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	err = outputQueue.Link(audioParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
	}

	go func() {
		ticker := time.NewTicker(time.Second * 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				state := pipeline.GetCurrentState()
				log.Debug(ctx, "pipeline state", "state", state)
			}
		}
	}()

	mp4sinkele, err := pipeline.GetElementByName("mp4sink")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	mp4sink := app.SinkFromElement(mp4sinkele)
	mp4sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "mp4sink EOSFunc")
			cancel()
		},
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}

func (mm *MediaManager) MKVPlayback(ctx context.Context, user string, w io.Writer) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	ctx = log.WithLogValues(ctx, "playbackID", uu.String())
	ctx, cancel := context.WithCancel(ctx)

	ctx = log.WithLogValues(ctx, "mediafunc", "MKVPlayback")

	pipelineSlice := []string{
		"matroskamux name=muxer streamable=true ! appsink name=mkvsink",
		"h264parse name=videoparse ! muxer.",
		"opusparse name=audioparse ! muxer.",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("failed to create GStreamer pipeline: %w", err)
	}

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	outputQueue, done, err := ConcatStream(ctx, pipeline, user, mm)
	if err != nil {
		return fmt.Errorf("failed to get output queue: %w", err)
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	err = outputQueue.Link(videoParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to video parse: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	err = outputQueue.Link(audioParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
	}

	go func() {
		ticker := time.NewTicker(time.Second * 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				state := pipeline.GetCurrentState()
				log.Debug(ctx, "pipeline state", "state", state)
			}
		}
	}()

	mkvsinkele, err := pipeline.GetElementByName("mkvsink")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	mkvsink := app.SinkFromElement(mkvsinkele)
	mkvsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			log.Warn(ctx, "mp4sink EOSFunc")
			cancel()
		},
	})

	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	pipeline.BlockSetState(gst.StateNull)

	return nil
}

func (mm *MediaManager) ParseSegmentMediaData(ctx context.Context, mp4bs []byte) (*model.SegmentMediaData, error) {
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ParseSegmentMediaData")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux name=demux ! fakesink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	var videoMetadata *model.SegmentMediadataVideo
	var audioMetadata *model.SegmentMediadataAudio

	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, bytes.NewReader(mp4bs)),
	})

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		caps := pad.GetCurrentCaps()
		if caps == nil {
			log.Warn(ctx, "Unable to get pad caps")
			cancel()
			return
		}

		structure := caps.GetStructureAt(0)
		if structure == nil {
			log.Warn(ctx, "Unable to get structure from caps")
			cancel()
			return
		}

		name := structure.Name()
		log.Debug(ctx, "Structure Name", "name", name)

		if name[:5] == "video" {
			videoMetadata = &model.SegmentMediadataVideo{}
			// Get some common video properties
			widthVal, _ := structure.GetValue("width")
			heightVal, _ := structure.GetValue("height")

			width, ok := widthVal.(int)
			if ok {
				videoMetadata.Width = width
			}
			height, ok := heightVal.(int)
			if ok {
				videoMetadata.Height = height
			}
			framerateVal, _ := structure.GetValue("framerate")
			framerateStr := fmt.Sprintf("%v", framerateVal)
			if framerateStr != "" {
				videoMetadata.Framerate = framerateStr
			}
		}

		if name[:5] == "audio" {
			audioMetadata = &model.SegmentMediadataAudio{}
			// Get some common audio properties
			rateVal, _ := structure.GetValue("rate")
			channelsVal, _ := structure.GetValue("channels")

			rate, ok := rateVal.(int)
			if ok {
				audioMetadata.Rate = rate
			}
			channels, ok := channelsVal.(int)
			if ok {
				audioMetadata.Channels = channels
			}
		}

		if videoMetadata != nil && audioMetadata != nil {
			cancel()
		}
	}

	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return nil, fmt.Errorf("error creating SegmentMetadata pipeline: %w", err)
	}
	demux.Connect("pad-added", onPadAdded)

	go func() {
		HandleBusMessages(ctx, pipeline)
		cancel()
	}()

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	<-ctx.Done()

	meta := &model.SegmentMediaData{
		Video: []*model.SegmentMediadataVideo{videoMetadata},
		Audio: []*model.SegmentMediadataAudio{audioMetadata},
	}

	pipeline.BlockSetState(gst.StateNull)

	return meta, nil
}
