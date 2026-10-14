package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-gst/go-glib/pkg/glib/v2"
	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test"
)

const HLSPlaylist = "stream.m3u8"

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

// newBufferFromBytes copies bs into a freshly allocated buffer.
func newBufferFromBytes(bs []byte) (*gst.Buffer, error) {
	buffer := gst.NewBufferAllocate(nil, uint(len(bs)), nil)
	info, ok := buffer.Map(gst.MapWrite)
	if !ok {
		return nil, fmt.Errorf("failed to map buffer for writing")
	}
	defer info.Unmap()
	copy(info.Data(), bs)
	return buffer, nil
}

// copyBufferData returns a copy of the buffer's contents that stays valid
// after the buffer is released.
func copyBufferData(buffer *gst.Buffer) ([]byte, error) {
	info, ok := buffer.Map(gst.MapRead)
	if !ok {
		return nil, fmt.Errorf("failed to map buffer for reading")
	}
	defer info.Unmap()
	return bytes.Clone(info.Data()), nil
}

// clockTimeDuration converts a gst.ClockTime to a time.Duration; ok is false
// when the time is unset (gst.ClockTimeNone).
func clockTimeDuration(t gst.ClockTime) (d time.Duration, ok bool) {
	if t == gst.ClockTimeNone {
		return 0, false
	}
	return time.Duration(t), true
}

// basic test to make sure gstreamer functionality is working
func SelfTest(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "mediafunc", "SelfTest")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	f, err := test.Files.Open("fixtures/sample-segment.mp4")
	if err != nil {
		return fmt.Errorf("failed to open test file: %w", err)
	}
	defer f.Close()
	bs, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read test file: %w", err)
	}

	pipeline := gst.NewPipeline("self-test").(gst.Pipeline)

	srcele := gst.ElementFactoryMakeWithProperties("appsrc", map[string]any{
		"name": "self-test-src",
	})
	if srcele == nil {
		return fmt.Errorf("failed to create appsrc element")
	}
	if !pipeline.Add(srcele) {
		return fmt.Errorf("failed to add appsrc to pipeline")
	}

	sinkele := gst.ElementFactoryMakeWithProperties("appsink", map[string]any{
		"name": "self-test-sink",
		"sync": false,
	})
	if sinkele == nil {
		return fmt.Errorf("failed to create appsink element")
	}
	if !pipeline.Add(sinkele) {
		return fmt.Errorf("failed to add appsink to pipeline")
	}

	if !srcele.Link(sinkele) {
		return fmt.Errorf("failed to link appsrc to appsink")
	}

	srcele.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: func(self app.AppSrc, _ uint) {
			buffer, err := newBufferFromBytes(bs)
			if err != nil {
				log.Error(ctx, "failed to build buffer", "error", err)
				cancel()
				return
			}
			self.PushBuffer(buffer)
			log.Debug(ctx, "ending stream")
			self.EndOfStream()
		},
	})

	output := &bytes.Buffer{}

	sinkele.(app.AppSink).SetCallbacks(&app.AppSinkCallbacks{
		NewSample: WriterNewSample(ctx, output),
		EOS: func(sink app.AppSink) {
			log.Debug(ctx, "EOSFunc")
			cancel()
		},
	})

	go func() {
		if err := HandleBusMessages(ctx, pipeline); err != nil {
			log.Debug(ctx, "handle bus messages failed", "error", err)
		}
		cancel()
	}()

	// Start the pipeline
	log.Debug(ctx, "setting pipeline to playing state")
	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline to playing state")
	}

	<-ctx.Done()

	if len(output.Bytes()) < 1 {
		return fmt.Errorf("got a zero-byte buffer from SelfTest")
	}

	if ret := pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone); ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline to null state")
	}

	return nil
}

const TestSrcWidth = 1280
const TestSrcHeight = 720
const QRSize = 256

type QRData struct {
	Now int64 `json:"now"`
}

func (mm *MediaManager) TestSource(ctx context.Context, ms MediaSigner) error {
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipelineSlice := []string{
		"h264parse name=videoparse",
		"compositor name=comp ! videoconvert ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast key-int-max=30 ! queue ! videoparse.",
		fmt.Sprintf(`videotestsrc is-live=true ! video/x-raw,format=AYUV,framerate=30/1,width=%d,height=%d ! comp.`, TestSrcWidth, TestSrcHeight),
		fmt.Sprintf("videobox border-alpha=0 top=-%d left=-%d name=box ! comp.", (TestSrcHeight/2)-(QRSize/2), (TestSrcWidth/2)-(QRSize/2)),
		"appsrc name=pngsrc ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! box.",
		"appsrc name=timetext ! pngdec ! videoconvert ! videorate ! video/x-raw,format=AYUV,framerate=1/1 ! comp.",
		"audiotestsrc ! audioconvert ! opusenc inband-fec=true perfect-timestamp=true bitrate=128000 ! queue ! opusparse name=audioparse",
	}

	pipelineElem, err := gst.ParseLaunch(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating TestSource pipeline: %w", err)
	}
	pipeline, ok := pipelineElem.(gst.Pipeline)
	if !ok {
		return fmt.Errorf("error creating TestSource pipeline: not a pipeline")
	}

	videoparse := pipeline.GetByName("videoparse")
	if videoparse == nil {
		return fmt.Errorf("videoparse element not found")
	}

	audioparse := pipeline.GetByName("audioparse")
	if audioparse == nil {
		return fmt.Errorf("audioparse element not found")
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	if !pipeline.Add(signer) {
		return fmt.Errorf("failed to add signer to pipeline")
	}

	if !videoparse.Link(signer) {
		return fmt.Errorf("link to signer failed")
	}
	if !audioparse.Link(signer) {
		return fmt.Errorf("link to signer failed")
	}

	pngele := pipeline.GetByName("pngsrc")
	if pngele == nil {
		return fmt.Errorf("pngsrc element not found")
	}

	pngele.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: func(self app.AppSrc, _ uint) {
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
			buffer, err := newBufferFromBytes(png)
			if err != nil {
				panic(err)
			}
			self.PushBuffer(buffer)
		},
	})
	tr, err := NewTextRenderer()
	if err != nil {
		return err
	}
	timetext := pipeline.GetByName("timetext")
	if timetext == nil {
		return fmt.Errorf("timetext element not found")
	}

	timetext.(app.AppSrc).SetCallbacks(&app.AppSrcCallbacks{
		NeedData: func(self app.AppSrc, _ uint) {
			aqt := aqtime.FromTime(time.Now())
			png, err := tr.GenerateImage(aqt.String(), "#ffffff", "#000000", 36)
			if err != nil {
				panic(err)
			}
			buffer, err := newBufferFromBytes(png)
			if err != nil {
				panic(err)
			}
			self.PushBuffer(buffer)
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		if ret := pipeline.BlockSetState(gst.StateNull, gst.ClockTimeNone); ret == gst.StateChangeFailure {
			log.Log(ctx, "failed to set pipeline state", "error", ret.String())
		}
		mainLoop.Quit()
	}()

	go func() {
		if err := HandleBusMessages(ctx, pipeline); err != nil {
			log.Log(ctx, "pipeline error", "error", err)
		}
		cancel()
	}()

	// Start the pipeline
	if ret := pipeline.SetState(gst.StatePlaying); ret == gst.StateChangeFailure {
		log.Log(ctx, "failed to set pipeline state", "error", ret.String())
	}

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		mainLoop.Run()
		log.Log(ctx, "main loop complete")
		return nil
	})

	return g.Wait()
}
