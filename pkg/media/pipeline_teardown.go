package media

import (
	"context"
	"runtime"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
)

// Shuts the pipeline down completely before allowing its Go wrapper to become
// unreachable. Call this only when the caller is finished with the pipeline, and
// it must not be used again after teardown returns.
func teardownPipeline(ctx context.Context, pipeline *gst.Pipeline) {
	// Teardown must complete synchronously. SetState may return while GStreamer
	// is still transitioning, allowing the Go wrapper's finalizer to release the
	// underlying object before shutdown finishes.
	if err := pipeline.BlockSetState(gst.StateNull); err != nil {
		log.Error(ctx, "failed to set pipeline state to null", "error", err)
	}

	// Keep the wrapper reachable through the full state transition so its
	// finalizer cannot release the underlying GStreamer object mid-teardown.
	runtime.KeepAlive(pipeline)
}
