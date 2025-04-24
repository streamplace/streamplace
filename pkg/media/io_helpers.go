package media

import (
	"context"
	"io"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

// ReaderNeedData is a function that reads from an io.Reader and pushes the data to a gstreamer source.
func ReaderNeedData(ctx context.Context, input io.Reader) func(self *app.Source, length uint) {
	bsCopy, err := io.ReadAll(input)
	if err != nil {
		panic(err)
	}
	return func(self *app.Source, length uint) {
		if ctx.Err() != nil {
			self.EndStream()
			return
		}
		buffer := gst.NewBufferWithSize(int64(len(bsCopy)))
		buffer.Map(gst.MapWrite).WriteData(bsCopy)
		defer buffer.Unmap()
		ret := self.PushBuffer(buffer)
		if ret != gst.FlowOK {
			log.Error(ctx, "failed to push buffer", "error", ret.String())
		} else {
			log.Debug(ctx, "pushed buffer", "length", len(bsCopy))
		}
	}
}

// WriterNewSample is a function that reads from a gstreamer sink and writes the data to an io.Writer.
func WriterNewSample(ctx context.Context, output io.Writer) func(sink *app.Sink) gst.FlowReturn {
	return func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowOK
		}

		// Retrieve the buffer from the sample.
		buffer := sample.GetBuffer()
		bs := buffer.Map(gst.MapRead).Bytes()
		defer buffer.Unmap()

		_, err := output.Write(bs)

		if err != nil {
			panic(err)
		}

		return gst.FlowOK
	}
}
