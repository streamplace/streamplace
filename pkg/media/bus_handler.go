package media

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
)

// ErrPipelineDone is a sentinel a pipeline element can raise (via
// pipeline.Error) to signal it finished its job and the bus handler should
// exit cleanly rather than treat it as a failure — e.g. the thumbnailer once
// it has grabbed its frame.
var ErrPipelineDone = errors.New("pipeline done")

func HandleBusMessages(ctx context.Context, pipeline *gst.Pipeline) error {
	return HandleBusMessagesCustom(ctx, pipeline, nil)
}

func HandleBusMessagesCustom(ctx context.Context, pipeline *gst.Pipeline, handler func(msg *gst.Message)) error {
	bus := pipeline.GetPipelineBus()
	defer runtime.KeepAlive(bus)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := bus.PopMessage(gst.ClockTime(time.Second * 1))
		if msg == nil {
			continue
		}
		if handler != nil {
			handler(msg)
		}
		messageType := msg.Type()
		runtime.KeepAlive(msg)
		switch messageType {
		case gst.MessageEOS: // When end-of-stream is received flush the pipeline and stop the main loop
			log.Debug(ctx, "got gst.MessageEOS, exiting")
			return nil
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			runtime.KeepAlive(msg)
			if err.Error() == fmt.Sprintf("%s: %s", ErrPipelineDone.Error(), ErrPipelineDone.Error()) {
				log.Debug(ctx, "got ErrPipelineDone, exiting")
				return nil
			}
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Debug(ctx, "gstreamer debug", "message", debug)
			}
			return fmt.Errorf("gstreamer error: %w", err)
		case gst.MessageElement:
			// this one is noisy and not useful
		default:
			log.Debug(ctx, "gstreamer bus message", "type", messageType)
		}
	}
}
