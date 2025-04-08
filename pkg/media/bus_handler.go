package media

import (
	"context"
	"fmt"
	"time"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
)

func HandleBusMessages(ctx context.Context, pipeline *gst.Pipeline) error {
	return HandleBusMessagesCustom(ctx, pipeline, nil)
}

func HandleBusMessagesCustom(ctx context.Context, pipeline *gst.Pipeline, handler func(msg *gst.Message)) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
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
			return nil
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Debug(ctx, "gstreamer debug", "message", debug)
			}
			return fmt.Errorf("gstreamer error: %w", err)
		default:
			log.Debug(ctx, msg.String())
		}
	}
}
