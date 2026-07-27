package discord

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadResponseBodyTruncates(t *testing.T) {
	body := bytes.Repeat([]byte("a"), maxResponseBodyLogBytes*10)
	out, err := readResponseBody(bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, maxResponseBodyLogBytes, len(out))
}

func TestReadResponseBodyStripsControlCharacters(t *testing.T) {
	out, err := readResponseBody(strings.NewReader("bad\x00body\x1b[31m\nwith\ttabs\r\n"))
	require.NoError(t, err)
	require.NotContains(t, out, "\x00")
	require.NotContains(t, out, "\x1b")
	require.NotContains(t, out, "\n")
	require.NotContains(t, out, "\t")
	require.NotContains(t, out, "\r")
	require.Contains(t, out, "badbody")
}

func TestReadResponseBodyKeepsPrintableUnicode(t *testing.T) {
	out, err := readResponseBody(strings.NewReader("error: quota exceeded 日本語"))
	require.NoError(t, err)
	require.Equal(t, "error: quota exceeded 日本語", out)
}
