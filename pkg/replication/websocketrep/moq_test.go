package websocketrep

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/moq"
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
		client, err = r.dialMoq(ctx, moqURL)
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
