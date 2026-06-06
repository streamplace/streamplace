package ingestframe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRoundTrip writes a mix of frame types/sizes and reads them back verbatim,
// then confirms a clean EOF at the boundary after the last frame.
func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	big := bytes.Repeat([]byte{0xAB}, 500_000)
	require.NoError(t, w.Answer("v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\n"))
	require.NoError(t, w.Segment([]byte("seg-one")))
	require.NoError(t, w.Segment(nil)) // zero-length segment is legal
	require.NoError(t, w.Segment(big))
	require.NoError(t, w.Event([]byte(`{"status":"active","message":"wrote 1234 bytes"}`)))
	require.NoError(t, w.Error("something broke"))
	require.NoError(t, w.End())

	r := NewReader(&buf)

	assertFrame := func(wantT Type, wantPayload []byte) {
		t.Helper()
		gotT, got, err := r.ReadFrame()
		require.NoError(t, err)
		require.Equal(t, wantT, gotT)
		require.Equal(t, wantPayload, got)
	}
	assertFrame(Answer, []byte("v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\n"))
	assertFrame(Segment, []byte("seg-one"))
	assertFrame(Segment, nil)
	assertFrame(Segment, big)
	assertFrame(Event, []byte(`{"status":"active","message":"wrote 1234 bytes"}`))
	assertFrame(Error, []byte("something broke"))
	assertFrame(End, nil)

	// Clean boundary after the last frame.
	_, _, err := r.ReadFrame()
	require.ErrorIs(t, err, io.EOF)
}

// TestTruncatedFrameIsUnexpectedEOF is the crash-vs-clean-end distinction the
// supervisor relies on: a worker that dies mid-segment must NOT look like a
// graceful end. CBOR's self-delimiting framing gives this for free — a byte
// string that declares more bytes than arrive surfaces as ErrUnexpectedEOF.
func TestTruncatedFrameIsUnexpectedEOF(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, NewWriter(&buf).Segment(bytes.Repeat([]byte{1}, 1000)))

	// Lop off the back half of the payload — an abrupt death mid-frame.
	full := buf.Bytes()
	torn := full[:len(full)-400]

	_, _, err := NewReader(bytes.NewReader(torn)).ReadFrame()
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestTornHeaderIsUnexpectedEOF: dying partway through the CBOR item head (here
// after the map header byte, before the first key) is also an abrupt death, not
// a clean boundary.
func TestTornHeaderIsUnexpectedEOF(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, NewWriter(&buf).End())
	torn := buf.Bytes()[:1] // just the map header; the rest never arrives

	_, _, err := NewReader(bytes.NewReader(torn)).ReadFrame()
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestGarbageRejected: a desynced/corrupt stream is caught, not mis-parsed. A
// complete-but-wrong-shaped CBOR item (a bare integer, not a frame map) must
// surface as a decode error distinct from EOF / a torn frame.
func TestGarbageRejected(t *testing.T) {
	_, _, err := NewReader(bytes.NewReader([]byte{0x01})).ReadFrame()
	require.Error(t, err)
	require.NotErrorIs(t, err, io.EOF)
	require.NotErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestConcurrentWritesDoNotInterleave: the worker emits segments from multiple
// goroutines (source signer + transcoder completion). Frames must stay whole.
func TestConcurrentWritesDoNotInterleave(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	const writers = 8
	const each = 50
	var wg sync.WaitGroup
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				// Distinct, self-identifying payloads so interleaving is detectable.
				payload := []byte(fmt.Sprintf("g%02d-i%02d-%s", g, i, bytes.Repeat([]byte("x"), i)))
				require.NoError(t, w.Segment(payload))
			}
		}(g)
	}
	wg.Wait()

	r := NewReader(&buf)
	var got []string
	for {
		typ, payload, err := r.ReadFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, Segment, typ)
		// Every payload must be one of the well-formed strings — a torn/interleaved
		// frame would fail this prefix shape or the count.
		require.Regexp(t, `^g\d\d-i\d\d-x*$`, string(payload))
		got = append(got, string(payload))
	}
	require.Len(t, got, writers*each, "every frame arrives exactly once, intact")

	// And every expected payload is present exactly once.
	want := make([]string, 0, writers*each)
	for g := 0; g < writers; g++ {
		for i := 0; i < each; i++ {
			want = append(want, fmt.Sprintf("g%02d-i%02d-%s", g, i, bytes.Repeat([]byte("x"), i)))
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	require.Equal(t, want, got)
}
