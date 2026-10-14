package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-gst/go-gst/pkg/gst"
	"stream.place/streamplace/pkg/log"
)

// ErrPipelineDone is a sentinel a pipeline element can post with
// gst.NewMessageError to signal it finished its job and the bus handler should
// exit cleanly rather than treat it as a failure.
var ErrPipelineDone = errors.New("pipeline done")

func HandleBusMessages(ctx context.Context, pipeline gst.Pipeline) error {
	return HandleBusMessagesCustom(ctx, pipeline, nil)
}

func HandleBusMessagesCustom(ctx context.Context, pipeline gst.Pipeline, handler func(msg *gst.Message)) error {
	bus := pipeline.GetBus()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := bus.TimedPop(gst.ClockTime(time.Second))
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
			debug, err := msg.ParseError()
			if err.Error() == ErrPipelineDone.Error() {
				log.Debug(ctx, "got ErrPipelineDone, exiting")
				return nil
			}
			log.Error(ctx, "gstreamer error", "error", err.Error())
			if debug != "" {
				log.Debug(ctx, "gstreamer debug", "message", debug)
			}
			return fmt.Errorf("gstreamer error: %w", err)
		case gst.MessageElement:
			// this one is noisy and not useful
		default:
			log.Debug(ctx, msg.String())
		}
	}
}
