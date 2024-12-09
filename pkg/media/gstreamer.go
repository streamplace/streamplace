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

	"aquareum.tv/aquareum/pkg/aqtime"
	"aquareum.tv/aquareum/pkg/log"
	"aquareum.tv/aquareum/test"
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/skip2/go-qrcode"
	"golang.org/x/sync/errgroup"
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
		self.PushBuffer(buffer)
	}
}

func WriterNewSample(ctx context.Context, output io.Writer) func(sink *app.Sink) gst.FlowReturn {
	return func(sink *app.Sink) gst.FlowReturn {
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
	}
}

func AddOpusToMKV(ctx context.Context, input io.Reader, output io.Writer) error {

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

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
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
		mainLoop.Quit()
	}()

	// Add a message handler to the pipeline bus, printing interesting information to the console.
	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {

		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			log.Debug(ctx, "got EOS")
			cancel()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Log(ctx, "gstreamer debug", "message", debug)
			}
			cancel()
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	mainLoop.Run()
	log.Log(ctx, "main loop complete")
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
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)
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
		"demux.audio_0 ! queue ! aacparse name=audioparse",
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

	audioparse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("error getting audioparse from ToHLS pipeline: %w", err)
	}
	err = audioparse.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking audioparse to splitmuxsink: %w", err)
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
		mainLoop.Quit()
	}()

	// Add a message handler to the pipeline bus, printing interesting information to the console.
	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {

		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			cancel()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Debug(ctx, "gstreamer debug", "message", debug)
			}
			cancel()
		case gst.MessageElement:
			structure := msg.GetStructure()
			name := structure.Name()
			if name == "splitmuxsink-fragment-opened" {
				runningTime, err := structure.GetValue("running-time")
				if err != nil {
					log.Warn(ctx, "splitmuxsink-fragment-opened error", "error", err)
					return true
				}
				runningTimeInt, ok := runningTime.(uint64)
				if !ok {
					log.Warn(ctx, "splitmuxsink-fragment-opened not a uint64")
					return true
				}
				m3u8.FragmentOpened(ctx, runningTimeInt)
			}
			if name == "splitmuxsink-fragment-closed" {
				runningTime, err := structure.GetValue("running-time")
				if err != nil {
					log.Warn(ctx, "splitmuxsink-fragment-closed error", "error", err)
					return true
				}
				runningTimeInt, ok := runningTime.(uint64)
				if !ok {
					log.Warn(ctx, "splitmuxsink-fragment-closed not a uint64")
					return true
				}
				m3u8.FragmentClosed(ctx, runningTimeInt)
			}
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	mainLoop.Run()
	log.Log(ctx, "main loop complete")

	return nil
}

func (mm *MediaManager) IngestStream(ctx context.Context, input io.Reader, ms *MediaSigner) error {
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

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {

		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			mainLoop.Quit()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Log(ctx, "gstreamer debug", "message", debug)
			}
			mainLoop.Quit()
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	mainLoop.Run()

	return nil
}

const TESTSRC_WIDTH = 1280
const TESTSRC_HEIGHT = 720
const QR_SIZE = 256

type QRData struct {
	Now int64 `json:"now"`
}

func (mm *MediaManager) TestSource(ctx context.Context, ms *MediaSigner) error {
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipelineSlice := []string{
		"h264parse name=videoparse",
		"compositor name=comp ! videoconvert ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast key-int-max=30 ! queue ! videoparse.",
		fmt.Sprintf(`videotestsrc is-live=true ! video/x-raw,format=AYUV,framerate=30/1,width=%d,height=%d ! comp.`, TESTSRC_WIDTH, TESTSRC_HEIGHT),
		fmt.Sprintf("videobox border-alpha=0 top=-%d left=-%d name=box ! comp.", (TESTSRC_HEIGHT/2)-(QR_SIZE/2), (TESTSRC_WIDTH/2)-(QR_SIZE/2)),
		"appsrc name=pngsrc ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! box.",
		"appsrc name=timetext ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! comp.",
		"audiotestsrc ! audioconvert ! fdkaacenc ! queue ! aacparse name=audioparse",
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

	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {

		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			cancel()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Log(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Log(ctx, "gstreamer debug", "message", debug)
			}
			cancel()
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

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
func (mm *MediaManager) SegmentAndSignElem(ctx context.Context, ms *MediaSigner) (*gst.Element, error) {
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

	elem.Connect("sink-added", func(split, sinkEle *gst.Element) {
		buf := &bytes.Buffer{}
		appsink := app.SinkFromElement(sinkEle)
		if appsink == nil {
			panic("appsink should not be nil")
		}
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, buf),
			EOSFunc: func(sink *app.Sink) {
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
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipelineSlice := []string{
		"appsrc name=appsrc ! qtdemux ! decodebin ! videoconvert ! videoscale ! video/x-raw,width=[1,200],height=[1,200],pixel-aspect-ratio=1/1 ! pngenc ! appsink name=appsink",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating TestSource pipeline: %w", err)
	}
	appsrc, err := pipeline.GetElementByName("appsrc")
	if err != nil {
		return err
	}

	src := app.SrcFromElement(appsrc)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedData(ctx, r),
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	appsink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		return err
	}

	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {

		case gst.MessageEOS: // When end-of-stream is received flush the pipeling and stop the main loop
			cancel()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Log(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Log(ctx, "gstreamer debug", "message", debug)
			}
			cancel()
		default:
			log.Debug(ctx, msg.String())
		}
		return true
	})

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, w),
		EOSFunc: func(sink *app.Sink) {
			cancel()
		},
	})

	pipeline.SetState(gst.StatePlaying)

	go func() {
		<-ctx.Done()
		pipeline.BlockSetState(gst.StateNull)
		mainLoop.Quit()
	}()

	mainLoop.Run()

	return nil
}
