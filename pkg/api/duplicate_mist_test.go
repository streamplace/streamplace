package api

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

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
