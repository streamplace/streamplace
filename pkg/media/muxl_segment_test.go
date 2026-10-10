package media

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

// endlessInput is an ingest body that never ends. io.Copy hands it the
// destination through WriteTo, so it sees the moment its writes start failing.
type endlessInput struct{ released chan struct{} }

func (endlessInput) Read(p []byte) (int, error) { return len(p), nil }

func (e endlessInput) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, 1024)
	var total int64
	for {
		n, err := w.Write(buf)
		total += int64(n)
		if err != nil {
			close(e.released)
			return total, err
		}
	}
}

// When the signer stops reading before the stream ends (a muxl error), the
// goroutine feeding it the ingest body must not stay blocked forever: every
// failed push would otherwise leak one.
func TestSignFMP4DirectReleasesInputWhenSignerStops(t *testing.T) {
	signErr := errors.New("signer failed")
	stopEarly := func(context.Context, io.Reader, chan *muxl.MuxlEvent) error { return signErr }
	input := endlessInput{released: make(chan struct{})}

	err := signFMP4Direct(context.Background(), &config.CLI{}, stopEarly, input, nil)
	require.ErrorIs(t, err, signErr)

	select {
	case <-input.released:
	case <-time.After(5 * time.Second):
		t.Fatal("the input copy is still blocked after signFMP4Direct returned")
	}
}
