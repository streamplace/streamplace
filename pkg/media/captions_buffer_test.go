package media

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaptionBufferBackpressureAndAbort(t *testing.T) {
	b := newIngestByteBuffer(context.Background())
	prefix := bytes.Repeat([]byte{'a'}, 32*1024*1024)
	_, err := b.Write(prefix)
	require.NoError(t, err)
	finished := make(chan error, 1)
	started := make(chan struct{})
	go func() { close(started); _, err := b.Write([]byte("tail")); finished <- err }()
	<-started
	select {
	case err := <-finished:
		t.Fatalf("full queue must backpressure, write returned %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	got := make([]byte, 64*1024)
	_, err = io.ReadFull(b, got)
	require.NoError(t, err)
	require.Equal(t, prefix[:len(got)], got)
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("reading must release a blocked producer")
	}
	require.NoError(t, b.CloseWithError(context.Canceled))
	got, err = io.ReadAll(b)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, got, "abort must release queued storage")
	_, err = b.Write([]byte("after abort"))
	require.ErrorIs(t, err, io.ErrClosedPipe)
}
