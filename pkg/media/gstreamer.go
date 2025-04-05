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
	"github.com/skip2/go-qrcode"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
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
