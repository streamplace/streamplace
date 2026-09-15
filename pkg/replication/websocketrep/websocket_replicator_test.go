package websocketrep

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func originView(streamer, wsURL string) *placestream.BroadcastDefs_BroadcastOriginView {
	return &placestream.BroadcastDefs_BroadcastOriginView{
		Record: &glex.LexiconTypeDecoder{Val: &placestream.BroadcastOrigin{
			Streamer:     streamer,
			WebsocketURL: &wsURL,
		}},
	}
}

// A pull whose origin refuses the first dials keeps trying and gets through
// once the origin is up; cancelling the pull ends it.
func TestPullRetriesUntilOriginAnswers(t *testing.T) {
	pullBackoffMin, pullBackoffMax = 5*time.Millisecond, 20*time.Millisecond
	var attempts atomic.Int32
	connected := make(chan struct{}, 1)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if attempts.Add(1) <= 3 {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			return
		}
		connected <- struct{}{}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	r := &WebsocketReplicator{conns: map[string]bool{}, latest: map[string]latestOrigin{}}
	const streamer = "did:plc:streamer"
	r.rememberOrigin(originView(streamer, "ws"+strings.TrimPrefix(srv.URL, "http")+"/xrpc/place.stream.live.subscribeSegments"), streamer)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.pull(ctx, streamer)
		close(done)
	}()
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("pull never got through to the origin")
	}
	require.GreaterOrEqual(t, attempts.Load(), int32(4), "three refusals, then the connection")
	require.True(t, r.hasConnection(streamer))
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pull did not stop on cancel")
	}
	require.False(t, r.hasConnection(streamer))
}

// An origin nobody has refreshed for a while is a stream that ended: the
// pull gives up instead of dialling forever.
func TestPullGivesUpOnStaleOrigin(t *testing.T) {
	pullBackoffMin, pullBackoffMax = 5*time.Millisecond, 20*time.Millisecond
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempts.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	r := &WebsocketReplicator{conns: map[string]bool{}, latest: map[string]latestOrigin{}}
	const streamer = "did:plc:streamer"
	r.rememberOrigin(originView(streamer, "ws"+strings.TrimPrefix(srv.URL, "http")+"/x"), streamer)
	r.latestMutex.Lock()
	r.latest[streamer] = latestOrigin{view: r.latest[streamer].view, seen: time.Now().Add(-originStaleAfter - time.Second)}
	r.latestMutex.Unlock()

	done := make(chan struct{})
	go func() {
		r.pull(context.Background(), streamer)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pull kept going on a stale origin")
	}
	require.Zero(t, attempts.Load(), "a stale origin is not dialled at all")
}
