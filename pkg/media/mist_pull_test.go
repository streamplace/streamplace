package media

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMistPullConnect drives mistPullConnect against a fake Mist: the first
// request 404s (the push hasn't started flowing yet — PUSH_REWRITE fires
// before Mist accepts the push), the second streams a chunked body. The
// connector must retry through the 404, then hand back the raw connection +
// buffered bytes + chunked flag such that WorkerInput reconstructs exactly the
// media bytes — the same contract the old hijacked-POST path provided.
func TestMistPullConnect(t *testing.T) {
	payload := make([]byte, 256*1024) // big enough to outsize any header-read buffering
	for i := range payload {
		payload[i] = byte(i)
	}

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
		for i := 0; i < len(payload); i += 4096 {
			end := min(i+4096, len(payload))
			_, err := w.Write(payload[i:end])
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
	require.Equal(t, payload, got, "prebuf + raw conn de-frames to the exact media bytes")
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

	go func() {
		time.Sleep(200 * time.Millisecond)
		l2, lerr := net.Listen("tcp", hostport)
		if lerr != nil {
			return // port raced away; the test will fail on the connect error
		}
		_ = http.Serve(l2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "media")
			w.(http.Flusher).Flush()
		}))
	}()

	conn, prebuf, chunked, err := mistPullConnect(context.Background(), hostport, "/late.mp4")
	require.NoError(t, err)
	defer conn.Close()
	got, err := io.ReadAll(WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, conn))
	require.NoError(t, err)
	require.Equal(t, []byte("media"), got)
}
