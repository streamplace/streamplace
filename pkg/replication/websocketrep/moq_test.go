package websocketrep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
)

// A peer pulling a stream through the listener gets the bus's segments for
// it, primed with the recent ones, in order, until the stream's session
// ends; a stream that is not live here is refused rather than waited on.
func TestMoqServesTheBus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	interval := moqLivenessInterval
	moqLivenessInterval = 10 * time.Millisecond
	t.Cleanup(func() { moqLivenessInterval = interval })

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := pc.LocalAddr().String()
	require.NoError(t, pc.Close())
	cert, err := moq.NewSelfSignedCert()
	require.NoError(t, err)
	tlsFiles, err := moq.ManagedTLSFiles(t.TempDir(), cert.Certificate)
	require.NoError(t, err)
	b := bus.NewBus()
	r := &WebsocketReplicator{bus: b, cli: &config.CLI{MoqAddr: addr}, MoqTLS: tlsFiles}
	served := make(chan error, 1)
	go func() { served <- r.serveMoQ(ctx) }()
	defer func() {
		cancel()
		<-served
	}()

	// Dialled the way a pull dials: the advertised URL, certificate pinned.
	moqURL := fmt.Sprintf("moqt://%s?%s=%s", addr, moq.CertHashParam, cert.Hash())
	var client *official.Client
	require.Eventually(t, func() bool {
		client, err = r.dialMoq(ctx, moqURL, nil)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "the listener comes up")
	defer client.Close()

	// The refusal reaches a puller by the time it subscribes.
	offline, err := requestMoqBroadcast(ctx, client, "did:plc:offline")
	if err == nil {
		err = consumeTrack(ctx, offline, MoqSourceTrack, func([]byte) error { return errors.New("a segment of a stream that is not live") })
	}
	require.Error(t, err, "a stream that is not live is refused")
	require.NotContains(t, err.Error(), "a segment of", "a stream that is not live is refused")
	require.NoError(t, ctx.Err(), "refused, not waited on")

	const streamer = "did:plc:live"
	segment := func(n byte) []byte { return []byte{0, 0, 0, 9, 'u', 'u', 'i', 'd', n} }
	b.PublishSegment(ctx, streamer, MoqSourceTrack, &bus.Seg{Muxl: segment(1), Published: true})
	broadcast, err := requestMoqBroadcast(ctx, client, streamer)
	require.NoError(t, err)

	got := make(chan []byte, 8)
	done := make(chan error, 1)
	go func() {
		done <- consumeTrack(ctx, broadcast, MoqSourceTrack, func(payload []byte) error {
			got <- payload
			return nil
		})
	}()
	next := func() []byte {
		select {
		case payload := <-got:
			return payload
		case err := <-done:
			t.Fatalf("the track ended early: %v", err)
		case <-ctx.Done():
			t.Fatal("no segment arrived")
		}
		return nil
	}
	require.Equal(t, segment(1), next(), "primed with the bus's recent segment")
	b.PublishSegment(ctx, streamer, MoqSourceTrack, &bus.Seg{Muxl: segment(2), Published: false})
	b.PublishSegment(ctx, streamer, MoqSourceTrack, &bus.Seg{Muxl: segment(3), Published: true})
	require.Equal(t, segment(3), next(), "an unpublished segment is not served")

	b.EndSession(ctx, streamer, MoqSourceTrack)
	select {
	case err := <-done:
		require.Error(t, err)
		require.False(t, errors.Is(err, context.DeadlineExceeded), "the broadcast ends with the stream")
	case <-ctx.Done():
		t.Fatal("the broadcast outlived its stream")
	}
}

// A stream this node ingests is announced to the peers it pushes to from
// its first segment, once, and retracted when its segments stop.
func TestLiveStreamsAreAnnounced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := NewWebsocketReplicator(&config.CLI{StreamSessionTimeout: 50 * time.Millisecond}, bus.NewBus(), nil, nil, nil, nil)
	r.group, ctx = errgroup.WithContext(ctx)
	announced, err := r.pushOrigin.Consume().Announced(official.AnnounceOptions{})
	require.NoError(t, err)
	defer announced.Cancel()

	r.touchLive(ctx, "did:plc:a")
	r.touchLive(ctx, "did:plc:a")
	a, err := announced.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, "did:plc:a", a.Prefix())
	require.True(t, a.Active())

	r.expireLive()
	require.Contains(t, r.live, "did:plc:a", "still live inside the session timeout")
	time.Sleep(80 * time.Millisecond)
	r.expireLive()
	a, err = announced.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, "did:plc:a", a.Prefix())
	require.False(t, a.Active(), "retracted, after one announcement")
}

// A peer this node pushes to gets each live stream announced on the session
// and its segments served for as long as the stream is live, not just the
// first one.
func TestMoqPushesLiveStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := pc.LocalAddr().String()
	require.NoError(t, pc.Close())
	cert, err := moq.NewSelfSignedCert()
	require.NoError(t, err)
	tlsFiles, err := moq.ManagedTLSFiles(t.TempDir(), cert.Certificate)
	require.NoError(t, err)
	// The peer: what acceptSession and acceptPush do, short of validation.
	srv, err := official.Listen(ctx, addr, official.WithTLSCert(tlsFiles.CertPath), official.WithTLSKey(tlsFiles.KeyPath))
	require.NoError(t, err)
	defer srv.Close()
	got := make(chan []byte, 8)
	go func() {
		for req, err := range srv.Requests(ctx) {
			if err != nil {
				return
			}
			pushed := official.NewOriginProducer()
			if req.SetConsume(pushed) != nil {
				return
			}
			sess, err := req.Accept(ctx)
			if err != nil {
				return
			}
			defer sess.Shutdown()
			announced, err := pushed.Consume().AnnouncedBroadcast("did:plc:a")
			if err != nil {
				return
			}
			broadcast, err := announced.Available(ctx)
			if err != nil {
				return
			}
			_ = consumeTrack(ctx, broadcast, MoqSourceTrack, func(payload []byte) error {
				got <- payload
				return nil
			})
			return
		}
	}()

	b := bus.NewBus()
	r := NewWebsocketReplicator(&config.CLI{StreamSessionTimeout: time.Minute}, b, nil, nil, nil, nil)
	r.group, ctx = errgroup.WithContext(ctx)
	segment := func(n byte) []byte { return []byte{0, 0, 0, 9, 'u', 'u', 'i', 'd', n} }
	b.PublishSegment(ctx, "did:plc:a", MoqSourceTrack, &bus.Seg{Muxl: segment(1), Published: true})
	r.touchLive(ctx, "did:plc:a")
	client, err := r.dialMoq(ctx, fmt.Sprintf("moqt://%s?%s=%s", addr, moq.CertHashParam, cert.Hash()), r.pushOrigin)
	require.NoError(t, err)
	defer client.Close()
	next := func() []byte {
		select {
		case payload := <-got:
			return payload
		case <-ctx.Done():
			t.Fatal("no segment arrived")
		}
		return nil
	}
	require.Equal(t, segment(1), next())
	b.PublishSegment(ctx, "did:plc:a", MoqSourceTrack, &bus.Seg{Muxl: segment(2), Published: true})
	require.Equal(t, segment(2), next(), "segments keep coming after the first")
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
