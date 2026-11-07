package director

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

// contender is a session on one node, contending for a role, with its
// start/stop calls recorded. Its start runs start, if set: nil from it
// takes the job up, an error fails to. A stop while the context the job
// was taken up with is still live is recorded as such: work running
// under it (a transcode) would outlive the role.
type contender struct {
	ss     *StreamSession
	cancel context.CancelFunc
	events chan string
}

func newContender(t *testing.T, state *statedb.StatefulDB, node string, start func() error) *contender {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	started := make(chan struct{})
	close(started)
	c := &contender{
		ss:     &StreamSession{cli: &config.CLI{ServerHost: node}, statefulDB: state, g: g, started: started},
		cancel: cancel,
		events: make(chan string, 16),
	}
	var roleCtx context.Context
	c.ss.contend(ctx, "did:plc:streamer", role{
		name: "record",
		start: func(rctx context.Context) error {
			roleCtx = rctx
			c.events <- node + " start"
			if start == nil {
				return nil
			}
			return start()
		},
		stop: func() {
			if roleCtx.Err() == nil {
				c.events <- node + " stop, role context live"
				return
			}
			c.events <- node + " stop"
		},
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

// leaseDB is a statedb for role tests, with leases short enough to watch.
func leaseDB(t *testing.T) *statedb.StatefulDB {
	roleLeaseTTL, roleLeaseRenew, roleRetryAfter = 150*time.Millisecond, 30*time.Millisecond, 300*time.Millisecond
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, mod)
	require.NoError(t, err)
	return state
}

// Two nodes carrying one stream: the first to ask does the job and keeps
// it while it renews; when it goes away the other takes over, at once on
// a clean exit (the lease is released) and within the TTL otherwise.
func TestRoleFailsOverBetweenNodes(t *testing.T) {
	state := leaseDB(t)

	a := newContender(t, state, "a.example", nil)
	a.expect(t, "a.example start", time.Second)
	b := newContender(t, state, "b.example", nil)
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
	c := newContender(t, state, "c.example", nil)
	c.quiet(t, roleLeaseTTL/2)
	c.expect(t, "c.example start", 3*roleLeaseTTL)
}

// A node that wins a job but cannot take it up (recording's lookups
// failed) hands the lease straight back, so a healthy peer does the job
// rather than nobody, and asks again later.
func TestRoleNotTakenUpGoesToAPeer(t *testing.T) {
	state := leaseDB(t)

	a := newContender(t, state, "a.example", func() error { return errors.New("lookup failed") })
	a.expect(t, "a.example start", time.Second)
	b := newContender(t, state, "b.example", nil)
	b.expect(t, "b.example start", roleLeaseRenew)
	a.quiet(t, 2*roleRetryAfter)

	// a still asks: once b is gone, it tries the job again.
	b.cancel()
	b.expect(t, "b.example stop", time.Second)
	a.expect(t, "a.example start", roleRetryAfter+3*roleLeaseRenew)
}

// A node that declines a job for this stream hands the lease back and
// stops asking for it.
func TestRoleDeclinedIsLeftToPeers(t *testing.T) {
	state := leaseDB(t)

	a := newContender(t, state, "a.example", func() error { return errRoleDeclined })
	a.expect(t, "a.example start", time.Second)
	b := newContender(t, state, "b.example", nil)
	b.expect(t, "b.example start", roleLeaseRenew)
	b.cancel()
	b.expect(t, "b.example stop", time.Second)
	a.quiet(t, 3*roleRetryAfter)
}

// A node whose taking up a job outlasts the lease it won (recording's
// lookups were slow) puts the job down at once rather than do it
// alongside a peer that took the lapsed lease meanwhile.
func TestRoleStartOutlastingTheLeaseIsPutDown(t *testing.T) {
	state := leaseDB(t)

	slow := true
	a := newContender(t, state, "a.example", func() error {
		if slow {
			slow = false
			time.Sleep(2 * roleLeaseTTL)
		}
		return nil
	})
	a.expect(t, "a.example start", time.Second)
	a.expect(t, "a.example stop", roleLeaseRenew/3)
	b := newContender(t, state, "b.example", nil)
	b.expect(t, "b.example start", roleLeaseRenew)
	a.quiet(t, 3*roleLeaseRenew)
}

// A node whose lease another holder has (here, taken over between two of
// its renewals) puts the job down, and ends the job's context first: work
// running under it, a transcode, stops with the role.
func TestRoleLostEndsItsContext(t *testing.T) {
	state := leaseDB(t)

	a := newContender(t, state, "a.example", nil)
	a.expect(t, "a.example start", time.Second)
	name, other := "stream:record:did:plc:streamer", "did:web:other.example"
	require.NoError(t, state.ReleaseLease(t.Context(), name, "did:web:a.example"))
	held, err := state.AcquireLease(t.Context(), name, other, time.Minute)
	require.NoError(t, err)
	require.True(t, held)
	a.expect(t, "a.example stop", 3*roleLeaseRenew)
}
