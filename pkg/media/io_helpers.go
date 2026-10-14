package media

import (
	"context"
	"errors"
	"io"

	"github.com/go-gst/go-gst/pkg/gst"
	app "github.com/go-gst/go-gst/pkg/gstapp"
	"stream.place/streamplace/pkg/log"
)

// ReaderNeedData is a function that reads from an io.Reader and pushes the data to a gstreamer source.
func ReaderNeedData(ctx context.Context, input io.Reader) func(self app.AppSrc, length uint) {
	bsCopy, err := io.ReadAll(input)
	if err != nil {
		log.Error(ctx, "error reading from input", "error", err)
	}
	wrote := false
	return func(self app.AppSrc, length uint) {
		if ctx.Err() != nil {
			self.EndOfStream()
			return
		}
		if wrote {
			log.Error(ctx, "ReaderNeedData: called after already wrote")
			ret := self.EndOfStream()
			if ret != gst.FlowOK {
				log.Error(ctx, "failed to end stream after already wrote", "error", ret.String())
			}
			return
		}
		wrote = true
		buffer, err := newBufferFromBytes(bsCopy)
		if err != nil {
			self.PostMessage(gst.NewMessageError(self, "error copying input", err))
			return
		}
		ret := self.PushBuffer(buffer)
		if ret != gst.FlowOK {
			log.Error(ctx, "failed to push buffer", "error", ret.String())
		} else {
			log.Debug(ctx, "pushed buffer", "length", len(bsCopy))
		}
		ret = self.EndOfStream()
		if ret != gst.FlowOK {
			log.Error(ctx, "failed to end stream", "error", ret.String())
		}
	}
}

// Different from ReaderNeedData in that it reads the data in chunks and pushes them to the source.
func ReaderNeedDataIncremental(ctx context.Context, input io.Reader) func(self app.AppSrc, length uint) {
	return func(self app.AppSrc, length uint) {
		if ctx.Err() != nil {
			self.EndOfStream()
			return
		}
		bs := make([]byte, length)
		read, err := input.Read(bs)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Error(ctx, "error reading from input", "error", err)
			self.PostMessage(gst.NewMessageError(self, "error reading from input", err))
			return
		}
		if read > 0 {
			toPush := bs
			if uint(read) < length {
				toPush = bs[:read]
			}
			buffer, err := newBufferFromBytes(toPush)
			if err != nil {
				self.PostMessage(gst.NewMessageError(self, "error copying input", err))
				return
			}
			self.PushBuffer(buffer)
		}
		if err != nil && errors.Is(err, io.EOF) {
			log.Debug(ctx, "EOF, ending stream", "length", read)
			self.EndOfStream()
			return
		}
	}
}

// WriterNewSample is a function that reads from a gstreamer sink and writes the data to an io.Writer.
func WriterNewSample(ctx context.Context, output io.Writer) func(sink app.AppSink) gst.FlowReturn {
	return func(sink app.AppSink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowOK
		}

		// Retrieve the buffer from the sample.
		buffer := sample.GetBuffer()
		mapped, ok := buffer.Map(gst.MapRead)
		if !ok {
			log.Error(ctx, "failed to map output buffer")
			return gst.FlowError
		}
		defer mapped.Unmap()

		_, err := output.Write(mapped.Data())

		if err != nil {
			log.Error(ctx, "error writing to output", "error", err)
			return gst.FlowError
		}

		return gst.FlowOK
	}
}
