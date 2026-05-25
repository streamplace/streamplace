package livehls

import (
	"context"
	"io"

	"stream.place/streamplace/pkg/muxl"
)

// Run drives the muxl segmenter over an fMP4 input and folds every event into
// an in-memory live HLS window, returning the populated Writer once the input
// is fully consumed — serve its MediaPlaylist/MasterPlaylist/InitSegment/
// SegmentData for playback.
func Run(ctx context.Context, input io.Reader, opts ...Option) (*Writer, error) {
	return drive(NewWriter(opts...), func(eventCh chan *muxl.MuxlEvent) error {
		return muxl.RunMuxlSegmenterEvents(ctx, input, eventCh)
	})
}

// RunSigned is the signed counterpart of Run: it drives muxl-sign's
// sign-segment over the fMP4 input, so each canonical segment folded into the
// window is C2PA/S2PA-signed ([c2pa-uuid][muxl-uuid][moof][mdat]). The
// playlists are identical in shape to Run; the only difference is the segment
// bytes carry the signing prefix. in.KeyPEM (or in.Sign) plus CertPEM and
// manifests must be set, matching muxl.RunMuxlSignSegment.
func RunSigned(ctx context.Context, input io.Reader, in muxl.SignerInput, opts ...Option) (*Writer, error) {
	return drive(NewWriter(opts...), func(eventCh chan *muxl.MuxlEvent) error {
		return muxl.RunMuxlSignSegment(ctx, input, in, nil, nil, eventCh)
	})
}

// drive runs a muxl event producer and feeds every event into w. The producer
// must send *MuxlEvents on the supplied channel and return when done; drive
// closes the channel after the producer returns. Every event is drained (even
// after an Observe error) so the producer never blocks on a full channel; the
// first Observe error is surfaced, otherwise the producer's error.
func drive(w *Writer, produce func(chan *muxl.MuxlEvent) error) (*Writer, error) {
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := produce(eventCh)
		close(eventCh)
		errCh <- err
	}()

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
