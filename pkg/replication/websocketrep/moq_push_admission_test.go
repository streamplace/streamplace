package websocketrep

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/config"
)

func TestMoqPushAdmissionLimits(t *testing.T) {
	r := &WebsocketReplicator{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sessions []map[string]*moqPushAdmission
	for session := range moqPushGlobalLimit / moqPushSessionLimit {
		running := map[string]*moqPushAdmission{}
		sessions = append(sessions, running)
		for i := range moqPushSessionLimit {
			_, ok := r.admitPush(running, fmt.Sprintf("did:web:s%d-%d.example", session, i), cancel)
			require.True(t, ok)
		}
		_, ok := r.admitPush(running, "did:web:overflow.example", cancel)
		require.False(t, ok, "per-session reservation cap")
	}
	extra := map[string]*moqPushAdmission{}
	_, ok := r.admitPush(extra, "did:web:global-overflow.example", cancel)
	require.False(t, ok, "global cap applies across independent sessions")
	path := "did:web:s0-0.example"
	entry := sessions[0][path]
	entry.cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	_, ok = r.admitPush(extra, "did:web:still-full.example", cancel)
	require.False(t, ok, "retraction cannot release a still-running worker's slot")
	r.releasePush(sessions[0], path, entry)
	r.releasePush(sessions[0], path, entry)
	require.Equal(t, moqPushGlobalLimit-1, r.pushAdmissions, "release is idempotent")
	_, ok = r.admitPush(extra, "did:web:replacement.example", cancel)
	require.True(t, ok)
}

// Exercise real official QUIC announcements: a syntactically invalid DID cannot
// reserve a worker, and a valid publisher with no source is evicted even though
// its connection and announcement remain open.
func TestMoqPushNoSourceDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	incoming := official.NewOriginProducer()
	server, err := official.Listen(ctx, "127.0.0.1:0", official.WithTLSGenerate("localhost"),
		official.WithServerPublishOrigin(official.NewOriginProducer()), official.WithServerSubscribeOrigin(incoming))
	require.NoError(t, err)
	defer server.Close()
	pins, err := server.CertFingerprints()
	require.NoError(t, err)
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(ctx) }()
	r := NewWebsocketReplicator(&config.CLI{WideOpen: true}, nil, nil, nil, nil, nil)
	pushDone := make(chan struct{})
	go func() { defer close(pushDone); r.acceptPushWithDeadline(ctx, incoming.Consume(), 500*time.Millisecond) }()
	published := official.NewOriginProducer()
	client, err := official.Dial(ctx, "moqt://"+server.LocalAddr(), official.WithTLSFingerprints(pins...),
		official.WithPublishOrigin(published), official.WithSubscribeOrigin(official.NewOriginProducer()), official.WithReconnect(false))
	require.NoError(t, err)
	defer client.Close()
	invalid, err := published.CreateBroadcast("did:not-a-did")
	require.NoError(t, err)
	defer invalid.Close()
	require.NoError(t, invalid.Announce(official.Route{}))
	valid, err := published.CreateBroadcast("did:web:no-source.example")
	require.NoError(t, err)
	defer valid.Close()
	// Keep track requests pending instead of rejecting them immediately.
	dynamic, err := valid.Dynamic()
	require.NoError(t, err)
	defer dynamic.Cancel()
	require.NoError(t, valid.Announce(official.Route{}))
	require.Eventually(t, func() bool { return r.ReceivesPush("did:web:no-source.example") }, 3*time.Second, 5*time.Millisecond)
	require.False(t, r.ReceivesPush("did:not-a-did"))
	require.Eventually(t, func() bool {
		r.pushedMu.Lock()
		defer r.pushedMu.Unlock()
		return r.pushAdmissions == 0 && len(r.pushed) == 0
	}, 3*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-pushDone:
	case <-time.After(2 * time.Second):
		t.Fatal("push workers did not stop")
	}
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("official listener did not stop")
	}
}
