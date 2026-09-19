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
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
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

// A pull never gives up: an origin nobody has refreshed for a long time is
// still dialled, just slowly.
func TestPullKeepsRetryingQuietOrigin(t *testing.T) {
	pullBackoffMin, pullBackoffMax, pullBackoffQuiet = 5*time.Millisecond, 20*time.Millisecond, 40*time.Millisecond
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
	r.latest[streamer] = latestOrigin{view: r.latest[streamer].view, seen: time.Now().Add(-originQuietAfter - time.Hour)}
	r.latestMutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.pull(ctx, streamer)
		close(done)
	}()
	require.Eventually(t, func() bool { return attempts.Load() >= 5 }, 5*time.Second, 5*time.Millisecond, "keeps dialling a quiet origin")
	cancel()
	<-done
}

// Rows from the shared statedb become origins to pull from, shaped like the
// firehose's, with the URL a node at that server DID advertises.
func TestOriginViewForRow(t *testing.T) {
	r := &WebsocketReplicator{cli: &config.CLI{ServerHost: "me.example", BehindHTTPSProxy: true}}
	view := r.originViewForRow(statedb.BroadcastOrigin{StreamerRepoDID: "did:plc:s", ServerDID: "did:web:origin.example", UpdatedAt: time.Now()})
	origin := view.Record.Val.(*placestream.BroadcastOrigin)
	require.Equal(t, "wss://origin.example/xrpc/place.stream.live.subscribeSegments?streamer=did%3Aplc%3As", *origin.WebsocketURL)
	require.Equal(t, "did:plc:s", origin.Streamer)
	require.Equal(t, "did:plc:s", view.Author.Did)
}
