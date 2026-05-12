package bdasl

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCID_DeterministicAndVerifiable(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog")
	cid := CID(data)
	require.NotEmpty(t, cid)
	require.True(t, strings.HasPrefix(cid, "b"))

	require.NoError(t, Verify(cid, data))
	require.Error(t, Verify(cid, append([]byte{}, data...)[:len(data)-1]))
}

func TestWriter_MatchesOneShot(t *testing.T) {
	data := bytes.Repeat([]byte("streamplace VOD content addressing!"), 1024)

	w := NewWriter()
	// Several writes of varying sizes — the streaming hasher must absorb
	// the same bytes as the one-shot variant for any chunking.
	chunks := [][]byte{data[:7], data[7:127], data[127:4321], data[4321:]}
	for _, c := range chunks {
		n, err := w.Write(c)
		require.NoError(t, err)
		require.Equal(t, len(c), n)
	}

	require.Equal(t, CID(data), w.CID())
}

func TestWriter_EmptyInput(t *testing.T) {
	w := NewWriter()
	require.Equal(t, CID(nil), w.CID())
}

func TestWriter_IsIOWriter(t *testing.T) {
	w := NewWriter()
	src := bytes.NewReader([]byte("hello world"))
	n, err := io.Copy(w, src)
	require.NoError(t, err)
	require.Equal(t, int64(len("hello world")), n)
	require.Equal(t, CID([]byte("hello world")), w.CID())
}
