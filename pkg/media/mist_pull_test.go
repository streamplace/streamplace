package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMistPullConnect drives mistPullConnect against a fake Mist: the first
// request 404s (the push hasn't started flowing yet — PUSH_REWRITE fires
// before Mist accepts the push), the second streams a chunked playable fMP4
// body. The connector must retry through the 404, then hand back the raw
// connection + buffered bytes + chunked flag such that WorkerInput
// reconstructs exactly the media bytes — the same contract the old
// hijacked-POST path provided, header bytes included.
func TestMistPullConnect(t *testing.T) {
	media := make([]byte, 256*1024) // big enough to outsize any header-read buffering
	for i := range media {
		media[i] = byte(i)
	}
	body := append(playableFMP4Header(2), media...)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/stream%2Bdid:test:abc_123.mp4", r.URL.EscapedPath())
		if calls.Add(1) == 1 {
			http.Error(w, "stream not ready", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		fl := w.(http.Flusher)
		// Stream in pieces with flushes so Go's server chunks the response —
		// the shape Mist's live output has.
		for i := 0; i < len(body); i += 4096 {
			end := min(i+4096, len(body))
			_, err := w.Write(body[i:end])
			require.NoError(t, err)
			fl.Flush()
		}
	}))
	defer srv.Close()

	oldGrace, oldBackoff := mistPullConnectGrace, mistPullRetryBackoff
	mistPullConnectGrace, mistPullRetryBackoff = 5*time.Second, 10*time.Millisecond
	defer func() { mistPullConnectGrace, mistPullRetryBackoff = oldGrace, oldBackoff }()

	hostport := srv.Listener.Addr().String()
	conn, prebuf, chunked, err := mistPullConnect(context.Background(), hostport, "/stream%2Bdid:test:abc_123.mp4")
	require.NoError(t, err)
	defer conn.Close()
	require.GreaterOrEqual(t, calls.Load(), int32(2), "connector retried through the 404")
	require.True(t, chunked, "streamed live body is chunked")

	got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, conn))
	require.NoError(t, err)
	require.Equal(t, body, got, "prebuf + raw conn de-frames to the exact media bytes")
}

// TestMistPullConnectTracklessThenPlayable is the prod race: Mist answers 200
// before the push has registered tracks, serving a moov with zero traks. The
// connector must treat that like a 404 — close, retry — and succeed on the
// next GET once the header is playable, with every byte reconstructed.
func TestMistPullConnectTracklessThenPlayable(t *testing.T) {
	media := bytes.Repeat([]byte{0xCD}, 64*1024)
	playable := append(playableFMP4Header(2), media...)
	trackless := playableFMP4Header(0)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		fl := w.(http.Flusher)
		if calls.Add(1) == 1 {
			_, err := w.Write(trackless)
			require.NoError(t, err)
			fl.Flush()
			return
		}
		for i := 0; i < len(playable); i += 4096 {
			end := min(i+4096, len(playable))
			_, err := w.Write(playable[i:end])
			require.NoError(t, err)
			fl.Flush()
		}
	}))
	defer srv.Close()

	oldGrace, oldBackoff := mistPullConnectGrace, mistPullRetryBackoff
	mistPullConnectGrace, mistPullRetryBackoff = 5*time.Second, 10*time.Millisecond
	defer func() { mistPullConnectGrace, mistPullRetryBackoff = oldGrace, oldBackoff }()

	conn, prebuf, chunked, err := mistPullConnect(context.Background(), srv.Listener.Addr().String(), "/trackless.mp4")
	require.NoError(t, err)
	defer conn.Close()
	require.GreaterOrEqual(t, calls.Load(), int32(2), "connector retried the zero-track header")
	require.True(t, chunked, "streamed live body is chunked")

	got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, conn))
	require.NoError(t, err)
	require.Equal(t, playable, got, "the successful GET's full stream reconstructs exactly")
}

// TestMistPullConnectTracklessGivesUp: a push that connects but never sends
// media keeps producing zero-track headers; the connector must give up within
// the grace with an error that names the actual problem.
func TestMistPullConnectTracklessGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(playableFMP4Header(0))
	}))
	defer srv.Close()

	oldGrace, oldBackoff := mistPullConnectGrace, mistPullRetryBackoff
	mistPullConnectGrace, mistPullRetryBackoff = 300*time.Millisecond, 20*time.Millisecond
	defer func() { mistPullConnectGrace, mistPullRetryBackoff = oldGrace, oldBackoff }()

	_, _, _, err := mistPullConnect(context.Background(), srv.Listener.Addr().String(), "/silent.mp4")
	require.Error(t, err)
	require.Contains(t, err.Error(), "never served a playable header")
}

// TestMistPullConnectStalledHeaderGivesUp covers the other flavor of
// trackless: Mist answers 200 but holds the moov (no header bytes arrive at
// all). The per-attempt header wait must fire so the attempt fails and the
// loop retries, giving up within the overall grace instead of hanging on the
// stalled body.
func TestMistPullConnectStalledHeaderGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(mp4Box("ftyp", []byte("isom")))
		w.(http.Flusher).Flush()
		// Hold the connection open without sending a moov, until the client
		// gives up and closes.
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	oldGrace, oldBackoff, oldWait := mistPullConnectGrace, mistPullRetryBackoff, mistPullHeaderWait
	mistPullConnectGrace, mistPullRetryBackoff, mistPullHeaderWait = 300*time.Millisecond, 20*time.Millisecond, 50*time.Millisecond
	defer func() {
		mistPullConnectGrace, mistPullRetryBackoff, mistPullHeaderWait = oldGrace, oldBackoff, oldWait
	}()

	start := time.Now()
	_, _, _, err := mistPullConnect(context.Background(), srv.Listener.Addr().String(), "/stalled.mp4")
	require.Error(t, err)
	require.Contains(t, err.Error(), "never served a playable header")
	require.Less(t, time.Since(start), 3*time.Second, "bounded by the grace, not the handler's hold")
}

// TestMistPullConnectGivesUp: a stream that never comes up must fail within
// the connect grace instead of retrying forever.
func TestMistPullConnectGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such stream", http.StatusNotFound)
	}))
	defer srv.Close()

	oldGrace, oldBackoff := mistPullConnectGrace, mistPullRetryBackoff
	mistPullConnectGrace, mistPullRetryBackoff = 300*time.Millisecond, 20*time.Millisecond
	defer func() { mistPullConnectGrace, mistPullRetryBackoff = oldGrace, oldBackoff }()

	_, _, _, err := mistPullConnect(context.Background(), srv.Listener.Addr().String(), "/nope.mp4")
	require.Error(t, err)
	require.Contains(t, err.Error(), "never came up")
}

// TestMistPullConnectRefusedThenUp: Mist itself may not even be listening yet
// (or between restarts); a refused connection is retried like a 404.
func TestMistPullConnectRefusedThenUp(t *testing.T) {
	// Reserve a port, then close the listener so the first dials are refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hostport := l.Addr().String()
	require.NoError(t, l.Close())

	oldGrace, oldBackoff := mistPullConnectGrace, mistPullRetryBackoff
	mistPullConnectGrace, mistPullRetryBackoff = 5*time.Second, 20*time.Millisecond
	defer func() { mistPullConnectGrace, mistPullRetryBackoff = oldGrace, oldBackoff }()

	body := playableFMP4Header(1)
	go func() {
		time.Sleep(200 * time.Millisecond)
		l2, lerr := net.Listen("tcp", hostport)
		if lerr != nil {
			return // port raced away; the test will fail on the connect error
		}
		_ = http.Serve(l2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(body)
			w.(http.Flusher).Flush()
		}))
	}()

	conn, prebuf, chunked, err := mistPullConnect(context.Background(), hostport, "/late.mp4")
	require.NoError(t, err)
	defer conn.Close()
	got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, conn))
	require.NoError(t, err)
	require.Equal(t, body, got)
}

// mp4Box builds one standard MP4 box: 4-byte big-endian size, 4-byte type,
// then the body.
func mp4Box(typ string, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(body)))
	copy(out[4:8], typ)
	copy(out[8:], body)
	return out
}

// mp4BoxLargesize builds a box with the 64-bit size form: size field 1, then
// the 8-byte largesize, then the body.
func mp4BoxLargesize(typ string, body []byte) []byte {
	out := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(out[0:4], 1)
	copy(out[4:8], typ)
	binary.BigEndian.PutUint64(out[8:16], uint64(16+len(body)))
	copy(out[16:], body)
	return out
}

// playableFMP4Header builds the header Mist's live .mp4 output serves on
// connect: ftyp, then moov with the given number of trak boxes, then the
// start of the fragment stream (a moof stub). A zero trak count is exactly
// what Mist serves when the push connected but no media has registered
// tracks yet — the shape that killed the ingest in prod.
func playableFMP4Header(traks int) []byte {
	var moov []byte
	moov = append(moov, mp4Box("mvhd", make([]byte, 4))...)
	for i := 0; i < traks; i++ {
		moov = append(moov, mp4Box("trak", mp4Box("tkhd", make([]byte, 4)))...)
	}
	out := mp4Box("ftyp", []byte("isom\x00\x00\x02\x00isom"))
	out = append(out, mp4Box("moov", moov)...)
	out = append(out, mp4Box("moof", make([]byte, 8))...)
	return out
}

func TestScanPlayableHeader(t *testing.T) {
	// The scan cap is generous everywhere except the dedicated cap test.
	const maxRead = int64(1 << 20)
	twoTrak := playableFMP4Header(2)

	skipFirst := mp4Box("ftyp", []byte("isom"))
	skipFirst = append(skipFirst, mp4Box("free", make([]byte, 1024))...)
	skipFirst = append(skipFirst, mp4Box("moov", mp4Box("trak", nil))...)

	large := mp4Box("ftyp", []byte("isom"))
	large = append(large, mp4BoxLargesize("moov", mp4Box("trak", nil))...)

	cases := []struct {
		name    string
		input   []byte
		maxRead int64
		tracks  int
		wantErr string
	}{
		{"two tracks", twoTrak, maxRead, 2, ""},
		{"one track", playableFMP4Header(1), maxRead, 1, ""},
		{"zero tracks", playableFMP4Header(0), maxRead, 0, ""},
		{"skip box before moov", skipFirst, maxRead, 1, ""},
		{"largesize moov", large, maxRead, 1, ""},
		{"ftyp only, no moov", mp4Box("ftyp", []byte("isom")), maxRead, 0, "EOF"},
		{"truncated mid moov", twoTrak[:50], maxRead, 0, "unexpected EOF"}, // moov body spans [28,80); cut inside it
		{"zero-size box", append(mp4Box("ftyp", nil), 0, 0, 0, 0, 'm', 'o', 'o', 'v'), maxRead, 0, "invalid"},
		{"cap exceeded before moov", append(mp4Box("free", make([]byte, 4096)), playableFMP4Header(1)...), 1024, 0, "no moov within"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks, err := scanPlayableHeader(bytes.NewReader(tc.input), tc.maxRead)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.tracks, tracks)
		})
	}
}

// TestAwaitPlayableHeader drives the conn-level check over a net.Pipe: it must
// consume only through the moov, return every raw byte it consumed as the new
// prebuf, and leave the rest of the stream intact — WorkerInput(prebuf, conn)
// reconstructs the exact media. Covered for both raw and chunked bodies, since
// the prebuf must stay in wire form (WorkerInput de-chunks).
func TestAwaitPlayableHeader(t *testing.T) {
	header := playableFMP4Header(2)
	media := bytes.Repeat([]byte{0xAB}, 4096) // post-header fragment bytes
	full := append(append([]byte{}, header...), media...)

	run := func(t *testing.T, chunked bool) {
		client, server := net.Pipe()
		defer client.Close()
		go func() {
			defer server.Close()
			if !chunked {
				_, _ = server.Write(full)
				return
			}
			cw := httputil.NewChunkedWriter(server)
			_, _ = cw.Write(full)
			_ = cw.Close()
		}()

		oldWait := mistPullHeaderWait
		mistPullHeaderWait = 5 * time.Second
		defer func() { mistPullHeaderWait = oldWait }()

		prebuf, err := awaitPlayableHeader(client, nil, chunked)
		require.NoError(t, err)
		require.NotEmpty(t, prebuf, "consumed header bytes come back as prebuf")

		got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, client))
		require.NoError(t, err)
		require.Equal(t, full, got, "prebuf + remaining conn reconstruct the exact stream")
	}
	t.Run("raw", func(t *testing.T) { run(t, false) })
	t.Run("chunked", func(t *testing.T) { run(t, true) })

	t.Run("existing prebuf is preserved", func(t *testing.T) {
		// The header read in tryMistGET can buffer body bytes past the headers;
		// those arrive as prebuf and must survive the check verbatim.
		split := 24
		client, server := net.Pipe()
		defer client.Close()
		go func() {
			defer server.Close()
			_, _ = server.Write(full[split:])
		}()
		prebuf, err := awaitPlayableHeader(client, full[:split], false)
		require.NoError(t, err)
		got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf}, client))
		require.NoError(t, err)
		require.Equal(t, full, got)
	})

	t.Run("zero tracks", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		go func() {
			defer server.Close()
			_, _ = server.Write(playableFMP4Header(0))
		}()
		_, err := awaitPlayableHeader(client, nil, false)
		require.ErrorIs(t, err, errNoTracks)
	})

	t.Run("deadline when no moov arrives", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		go func() {
			defer server.Close()
			_, _ = server.Write(mp4Box("ftyp", []byte("isom")))
			time.Sleep(2 * time.Second) // stall past the (shortened) wait
		}()
		oldWait := mistPullHeaderWait
		mistPullHeaderWait = 100 * time.Millisecond
		defer func() { mistPullHeaderWait = oldWait }()
		start := time.Now()
		_, err := awaitPlayableHeader(client, nil, false)
		require.Error(t, err)
		require.Less(t, time.Since(start), time.Second, "deadline unblocks the wait")
	})
}
