package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShadowIdleReaderAllowsConsumerWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reader := newShadowIdleReader(bytes.NewReader([]byte{3}), cancel)
		defer reader.stop()
		var buf [1]byte
		n, err := reader.Read(buf[:])
		require.NoError(t, err)
		require.Equal(t, 1, n)

		// A caller processing bytes is not waiting on idle publisher input.
		time.Sleep(2 * RTMPTimeout)
		require.NoError(t, ctx.Err())
		n, err = reader.Read(buf[:])
		require.ErrorIs(t, err, io.EOF)
		require.Zero(t, n)
		time.Sleep(2 * RTMPTimeout)
		require.NoError(t, ctx.Err(), "publisher EOF must allow downstream drain")
	})
}

func TestShadowIdleReaderStaleCallbackHonorsPendingRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input, publisher := io.Pipe()
		defer input.Close()
		defer publisher.Close()
		stopInput := context.AfterFunc(ctx, func() { _ = input.Close() })
		defer stopInput()
		reader := newShadowIdleReader(input, cancel)
		defer reader.stop()
		result := make(chan error, 1)
		go func() {
			var buf [1]byte
			_, err := reader.Read(buf[:])
			result <- err
		}()
		synctest.Wait()
		time.Sleep(RTMPTimeout / 2)
		// A previous timer callback can already be queued when a new Read starts.
		reader.expire()
		require.NoError(t, ctx.Err(), "an old callback must not shorten this Read")
		time.Sleep(RTMPTimeout / 2)
		require.ErrorIs(t, <-result, errShadowIdleTimeout)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	})
}

func TestDuplicateMistIdleHandshakeClosesOnlyShadowInput(t *testing.T) {
	input, publisher := io.Pipe()
	defer input.Close()
	defer publisher.Close()
	result := make(chan error, 1)
	go func() { result <- RunDuplicateMistWorker(t.Context(), input, io.Discard) }()
	select {
	case err := <-result:
		require.ErrorIs(t, err, errShadowIdleTimeout)
	case <-time.After(RTMPTimeout + 5*time.Second):
		t.Fatal("idle shadow handshake did not exit")
	}
}

func TestDuplicateMistRejectsTruncatedHandshake(t *testing.T) {
	for name, tc := range map[string]struct {
		input []byte
		want  error
	}{
		"no handshake":              {nil, io.EOF},
		"truncated plain handshake": {[]byte{3, 0, 0}, io.ErrUnexpectedEOF},
	} {
		t.Run(name, func(t *testing.T) {
			err := RunDuplicateMistWorker(t.Context(), bytes.NewReader(tc.input), io.Discard)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

// Upstream parser errors can print entire AMF connect/publish commands.
func TestShadowProtocolErrorsNeverExposeStreamKey(t *testing.T) {
	const key = "zSECRET-STREAM-KEY"
	for _, err := range []error{
		fmt.Errorf("invalid publish arguments: %s", key),
		fmt.Errorf("publish %s: %w", key, io.EOF),
		fmt.Errorf("publish %s: %w", key, io.ErrUnexpectedEOF),
	} {
		require.NotContains(t, shadowProtocolError("accept", err).Error(), key)
	}
}
