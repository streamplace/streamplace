package media

import (
	"context"
	"time"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
)

func HandleBusMessages(ctx context.Context, pipeline *gst.Pipeline) {
	HandleBusMessagesCustom(ctx, pipeline, nil)
}

func HandleBusMessagesCustom(ctx context.Context, pipeline *gst.Pipeline, handler func(msg *gst.Message)) {
	for {
		if ctx.Err() != nil {
			return
		}
		msg := pipeline.GetPipelineBus().PopMessage(gst.ClockTime(time.Second * 1))
		if msg == nil {
			continue
		}
		if handler != nil {
			handler(msg)
		}
		switch msg.Type() {
		case gst.MessageEOS: // When end-of-stream is received flush the pipeline and stop the main loop
			log.Debug(ctx, "got gst.MessageEOS, exiting")
			return
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Debug(ctx, "gstreamer debug", "message", debug)
			}
			return
		default:
			log.Debug(ctx, msg.String())
		}
	}
}
