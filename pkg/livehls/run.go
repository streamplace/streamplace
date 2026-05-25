package livehls

import (
	"context"
	"io"

	"stream.place/streamplace/pkg/muxl"
)

// Run drives the muxl segmenter over an fMP4 input and assembles a live HLS
// presentation: every muxl event is fed into a Writer, which appends the init
// and each canonical segment to out (a growing fMP4) and maintains the
// per-track byte-range index. It returns the populated Writer once the input
// is fully consumed — call its MediaPlaylist/MasterPlaylist/Track methods to
// serve playback.
//
// This is the unsigned path. For signed live HLS, drive a Writer from
// muxl.RunMuxlSignSegment's eventCh instead: the signed-segment events have
// the same shape (each segment's bytes simply carry a leading c2pa-uuid box,
// which only changes the byte length the index records).
func Run(ctx context.Context, input io.Reader, out io.Writer, opts ...Option) (*Writer, error) {
	w := NewWriter(out, opts...)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := muxl.RunMuxlSegmenterEvents(ctx, input, eventCh)
		close(eventCh)
		errCh <- err
	}()

	// Drain every event (even after an Observe error) so the wasm goroutine
	// never blocks on a full eventCh; surface the first Observe error.
	var obsErr error
	for ev := range eventCh {
		if obsErr == nil {
			obsErr = w.Observe(ev)
		}
	}
	runErr := <-errCh
	if obsErr != nil {
		return w, obsErr
	}
	return w, runErr
}
