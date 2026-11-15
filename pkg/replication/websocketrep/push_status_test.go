package websocketrep

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
)

func TestPushStatusTransitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := pc.LocalAddr().String()
	require.NoError(t, pc.Close())
	cert, err := moq.NewSelfSignedCert()
	require.NoError(t, err)
	tlsFiles, err := moq.ManagedTLSFiles(t.TempDir(), cert.Certificate)
	require.NoError(t, err)
	require.NoError(t, tlsFiles.Sync())
	srv, err := official.Listen(ctx, addr, official.WithTLSCert(tlsFiles.CertPath), official.WithTLSKey(tlsFiles.KeyPath))
	require.NoError(t, err)
	defer srv.Close()
	go func() {
		for req, err := range srv.Requests(ctx) {
			if err != nil {
				return
			}
			if err := req.SetConsume(official.NewOriginProducer()); err != nil {
				return
			}
			sess, err := req.Accept(ctx)
			if err != nil {
				return
			}
			defer sess.Shutdown()
			_ = sess.Closed(ctx)
			return
		}
	}()

	var ready atomic.Bool
	moqURL := fmt.Sprintf("moqt://%s?%s=%s", addr, moq.CertHashParam, cert.Hash())
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		out := placestream.BroadcastGetBroadcaster_Output{}
		if ready.Load() {
			out.MoqURL = &moqURL
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer peer.Close()
	r := NewWebsocketReplicator(&config.CLI{Peers: []string{peer.URL}}, nil, nil, nil, nil, nil)
	r.live["did:plc:streamer"] = &liveStream{}
	initial := r.PushStatus()
	require.Equal(t, peer.URL, initial.Peers[0].Peer)
	require.False(t, initial.Peers[0].Connected)
	require.Zero(t, initial.Peers[0].ConnectedAt)
	require.Equal(t, []string{"did:plc:streamer"}, initial.Live)

	err = r.openPush(ctx, peer.URL)
	require.ErrorContains(t, err, "no moq listener")
	require.Equal(t, err.Error(), r.PushStatus().Peers[0].LastError)
	ready.Store(true)
	done := make(chan error, 1)
	go func() { done <- r.openPush(ctx, peer.URL) }()
	require.Eventually(t, func() bool { return r.PushStatus().Peers[0].Connected }, 5*time.Second, 10*time.Millisecond)
	connected := r.PushStatus().Peers[0]
	require.Equal(t, "moqt://"+addr, connected.MoqURL)
	require.NotZero(t, connected.ConnectedAt)
	require.Empty(t, connected.LastError)

	cancel()
	select {
	case err = <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("push session did not end")
	}
	disconnected := r.PushStatus().Peers[0]
	require.False(t, disconnected.Connected)
	require.Equal(t, connected.ConnectedAt, disconnected.ConnectedAt)
	require.Equal(t, connected.MoqURL, disconnected.MoqURL)
	require.Equal(t, err.Error(), disconnected.LastError)
}
