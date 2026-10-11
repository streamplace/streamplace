package websocketrep

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
)

// The announced inventory is the streams with recent local segments: a
// streamer appears on its first segment, once, and is retracted when its
// segments stop; a listener joining late gets the current set first.
func TestLiveSetAnnounces(t *testing.T) {
	l := newLiveSet(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	early := l.announce(ctx, "")
	l.touch("did:plc:a")
	l.touch("did:plc:a")
	late := l.announce(ctx, "did:plc:")
	other := l.announce(ctx, "did:web:")

	require.Equal(t, moq.Announcement{Path: "did:plc:a", Active: true}, <-early)
	require.Equal(t, moq.Announcement{Path: "did:plc:a", Active: true}, <-late)
	require.Empty(t, other, "a listener only hears paths under its prefix")

	time.Sleep(80 * time.Millisecond)
	l.expire()
	require.Equal(t, moq.Announcement{Path: "did:plc:a"}, <-early)
	require.Equal(t, moq.Announcement{Path: "did:plc:a"}, <-late)
	require.Empty(t, early, "one announcement per change")

	cancel()
	_, open := <-early
	require.False(t, open, "the channel closes with the listener's context")
}

// A --peers entry naming a node's web origin resolves to the MoQ URL that
// node currently reports; one with no listener is an error to retry, not a
// session to open.
func TestResolvePeer(t *testing.T) {
	var moqURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/xrpc/place.stream.broadcast.getBroadcaster", req.URL.Path)
		out := placestream.BroadcastGetBroadcaster_Output{Broadcaster: "did:web:peer"}
		if moqURL != "" {
			out.MoqURL = &moqURL
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	r := &WebsocketReplicator{}

	_, err := r.resolvePeer(context.Background(), srv.URL)
	require.ErrorContains(t, err, "no moq listener")

	moqURL = "moqt://127.0.0.1:38443?certhash=ab"
	got, err := r.resolvePeer(context.Background(), srv.URL+"/ignored?x=1")
	require.NoError(t, err)
	require.Equal(t, moqURL, got)

	got, err = r.resolvePeer(context.Background(), "moqt://peer.example:38443?certhash=cd")
	require.NoError(t, err)
	require.Equal(t, "moqt://peer.example:38443?certhash=cd", got)

	_, err = r.resolvePeer(context.Background(), "ftp://peer.example")
	require.Error(t, err)
}
