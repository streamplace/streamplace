package director

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

// contender is a session on one node, contending for a role, with its
// start/stop calls recorded.
type contender struct {
	ss     *StreamSession
	cancel context.CancelFunc
	events chan string
}

func newContender(t *testing.T, state *statedb.StatefulDB, node string) *contender {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	started := make(chan struct{})
	close(started)
	c := &contender{
		ss:     &StreamSession{cli: &config.CLI{ServerHost: node}, statefulDB: state, g: g, started: started},
		cancel: cancel,
		events: make(chan string, 16),
	}
	c.ss.contend(ctx, "did:plc:streamer", role{
		name:  "record",
		start: func(context.Context) { c.events <- node + " start" },
		stop:  func() { c.events <- node + " stop" },
	})
	t.Cleanup(func() {
		cancel()
		_ = g.Wait()
	})
	return c
}

func (c *contender) expect(t *testing.T, event string, within time.Duration) {
	t.Helper()
	select {
	case got := <-c.events:
		require.Equal(t, event, got)
	case <-time.After(within):
		t.Fatalf("no %q within %s", event, within)
	}
}

func (c *contender) quiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case got := <-c.events:
		t.Fatalf("unexpected %q", got)
	case <-time.After(d):
	}
}

// Two nodes carrying one stream: the first to ask does the job and keeps
// it while it renews; when it goes away the other takes over, at once on
// a clean exit (the lease is released) and within the TTL otherwise.
func TestRoleFailsOverBetweenNodes(t *testing.T) {
	roleLeaseTTL, roleLeaseRenew = 150*time.Millisecond, 30*time.Millisecond
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)

	a := newContender(t, state, "a.example")
	a.expect(t, "a.example start", time.Second)
	b := newContender(t, state, "b.example")
	b.quiet(t, 3*roleLeaseTTL)

	// a ends cleanly: its stop runs and b has the job before the TTL.
	a.cancel()
	a.expect(t, "a.example stop", time.Second)
	b.expect(t, "b.example start", roleLeaseTTL)

	// b dies without releasing: its lease stays on the books until it
	// lapses, and a new node gets the job then, not before. (A clean
	// cancel would release it, so the dead node's lease is put back, once
	// that release, which follows the stop, is done.)
	b.cancel()
	b.expect(t, "b.example stop", time.Second)
	require.NoError(t, b.ss.g.Wait())
	held, err := state.AcquireLease(t.Context(), "stream:record:did:plc:streamer", "did:web:b.example", roleLeaseTTL)
	require.NoError(t, err)
	require.True(t, held)
	c := newContender(t, state, "c.example")
	c.quiet(t, roleLeaseTTL/2)
	c.expect(t, "c.example start", 3*roleLeaseTTL)
}
