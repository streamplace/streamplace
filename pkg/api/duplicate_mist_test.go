package api

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDuplicateMistIdleHandshakeClosesOnlyShadowInput(t *testing.T) {
	input, publisher := io.Pipe()
	defer input.Close()
	defer publisher.Close()
	result := make(chan error, 1)
	go func() { result <- RunDuplicateMistWorker(t.Context(), input) }()
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
			err := RunDuplicateMistWorker(t.Context(), bytes.NewReader(tc.input))
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
