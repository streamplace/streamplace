package atproto

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/reposync"
)

// TestBackfillWindowedHistory is the windowed backfill end to end against the
// reference PDS: a first sync reads the account's configuration and its recent
// chat, and the deepening ladder fetches the rest of its history afterwards,
// one window at a time.
//
// The old messages are planted with explicit TID rkeys, which is how they would
// have arrived months ago -- an rkey is a timestamp, so writing one is the only
// way to have an old record in a repo created a second ago.
func TestBackfillWindowedHistory(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	now := time.Now()
	// Configuration-shaped records: never windowed, always synced.
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	// Recent chat, inside the initial window (the PDS mints a TID for now).
	createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "today"))
	// History, at ages that land on distinct rungs of the ladder.
	plant := func(age time.Duration, text string) {
		t.Helper()
		createBackfillRecord(t, user, "place.stream.chat.message",
			reposync.TIDForTime(now.Add(-age)), chatMessageRecord(user.DID, text))
	}
	plant(3*24*time.Hour, "three days ago")
	plant(20*24*time.Hour, "twenty days ago")
	plant(200*24*time.Hour, "two hundred days ago")

	countMessages := func() int {
		messages, err := mod.MostRecentChatMessages(user.DID)
		require.NoError(t, err)
		return len(messages)
	}

	// Wait for the PDS to have committed everything, by walking the whole
	// (unwindowed) range until all five records are there. Doing this before
	// the sync means a short count later is a windowing decision, not a race.
	require.NoError(t, untilNoErrors(t, func() error {
		paths, err := walkAll(ctx, dev, user.DID, backfillRanges(""))
		if err != nil {
			return err
		}
		if len(paths) != 5 {
			return fmt.Errorf("PDS has %d records, want 5", len(paths))
		}
		return nil
	}), "waiting for the repo to settle")

	published := watchBus(t, atsync.Bus, user.DID)

	// The shallow sync: everything unwindowed, plus one day of chat.
	repo, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, repo.Version, "a shallow sync still records the rev it read")
	require.NotEmpty(t, repo.BackfillFloor, "a shallow sync records how far back it went")
	require.False(t, repo.BackfillDone, "history is not synced yet")
	floorTime, err := reposync.TimeForTID(repo.BackfillFloor)
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(-InitialWindow), floorTime, time.Minute)

	profile, err := mod.GetChatProfile(ctx, user.DID)
	require.NoError(t, err)
	require.NotNil(t, profile, "unwindowed collections are synced in full on first contact")
	require.Equal(t, 1, countMessages(), "only today's message is inside the initial window")

	// Now the ladder. Each rung reaches further back, and a message shows up
	// exactly when the window covering its rkey is walked -- not before.
	wantAfterRung := []int{
		2, // [7d, 1d)         -- the three-day-old message
		3, // [30d, 7d)        -- the twenty-day-old message
		3, // [180d, 30d)      -- nothing lives here
		4, // [genesis, 180d)  -- the two-hundred-day-old message
	}
	var done bool
	var floor string
	for rung, want := range wantAfterRung {
		require.False(t, done, "the ladder finished early at rung %d", rung)
		previous := floor
		done, floor, err = atsync.DeepenRepo(ctx, user.DID)
		require.NoError(t, err, "rung %d", rung)
		require.Equal(t, want, countMessages(), "message count after rung %d", rung)
		if !done {
			// The floor a window reports is what the sweep's horizon is made
			// of, so it has to be the watermark that was actually written, and
			// it has to keep reaching further back.
			stored, err := mod.GetRepo(user.DID)
			require.NoError(t, err)
			require.Equal(t, stored.BackfillFloor, floor, "rung %d reports the watermark it wrote", rung)
			if previous != "" {
				// TIDs sort by time, so each rung's watermark is smaller.
				require.Less(t, floor, previous, "rung %d reaches further back than the last", rung)
			}
		}
	}
	require.True(t, done, "the last window bottoms out the collection")

	stored, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.True(t, stored.BackfillDone, "the watermark is durable")
	require.NotEmpty(t, stored.Version)

	// Every message reached the chat bus exactly once, even though the window
	// boundaries mean the walker re-emitted records it had already seen.
	require.Equal(t, 4, published(), "each message should be published once")

	// And a repo that is done is done: another sweep costs nothing and says
	// nothing.
	again, _, err := atsync.DeepenRepo(ctx, user.DID)
	require.NoError(t, err)
	require.True(t, again)
	require.NoError(t, atsync.Sweep(ctx))
	require.Equal(t, 4, countMessages(), "a second sweep must not duplicate anything")
	require.Equal(t, 4, published(), "a second sweep must not re-publish anything")
}

// TestSweepShallowThenDeepens drives the whole sweep over two accounts in the
// two states a real node has after a deploy: one it has never synced, and one
// carrying a row from before the watermark existed.
func TestSweepShallowThenDeepens(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)
	now := time.Now()

	fresh := dev.CreateAccount(t)
	createBackfillRecord(t, fresh, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	createBackfillRecord(t, fresh, "place.stream.chat.message", "", chatMessageRecord(fresh.DID, "fresh today"))
	createBackfillRecord(t, fresh, "place.stream.chat.message",
		reposync.TIDForTime(now.Add(-90*24*time.Hour)), chatMessageRecord(fresh.DID, "fresh long ago"))

	legacy := dev.CreateAccount(t)
	createBackfillRecord(t, legacy, "place.stream.chat.message", "", chatMessageRecord(legacy.DID, "legacy today"))
	createBackfillRecord(t, legacy, "place.stream.chat.message",
		reposync.TIDForTime(now.Add(-300*24*time.Hour)), chatMessageRecord(legacy.DID, "legacy ages ago"))

	require.NoError(t, untilNoErrors(t, func() error {
		for did, want := range map[string]int{fresh.DID: 3, legacy.DID: 2} {
			paths, err := walkAll(ctx, dev, did, backfillRanges(""))
			if err != nil {
				return err
			}
			if len(paths) != want {
				return fmt.Errorf("repo %s has %d records, want %d", did, len(paths), want)
			}
		}
		return nil
	}), "waiting for the repos to settle")

	// The fresh account is known but unsynced: a placeholder row, exactly what
	// the firehose writes when it first sees a record from a stranger.
	require.NoError(t, atsync.StatefulDB.AddRepo(fresh.DID))
	// The legacy account has a completed sync from a build that had never heard
	// of a backfill window: a version, no floor, not done.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     legacy.DID,
		PDS:     dev.PDSURL,
		Handle:  legacy.Handle,
		Version: "3lpretend0000",
	}))
	require.NoError(t, atsync.StatefulDB.AddRepo(legacy.DID))

	require.NoError(t, atsync.Sweep(ctx))

	for _, did := range []string{fresh.DID, legacy.DID} {
		stored, err := mod.GetRepo(did)
		require.NoError(t, err)
		require.NotEmpty(t, stored.Version, "%s should have been synced", did)
		require.True(t, stored.BackfillDone, "%s should have been deepened to the end", did)
		messages, err := mod.MostRecentChatMessages(did)
		require.NoError(t, err, did)
		require.Len(t, messages, 2, "both messages of %s should be indexed", did)
	}
	// The fresh account went through the full backfill, so its unwindowed
	// records are there too; the legacy one was only ever deepened, which by
	// design touches nothing but the windowed collections.
	profile, err := mod.GetChatProfile(ctx, fresh.DID)
	require.NoError(t, err)
	require.NotNil(t, profile)

	// Idempotent: a second sweep is a few head fetches and nothing else.
	require.NoError(t, atsync.Sweep(ctx))
	for _, did := range []string{fresh.DID, legacy.DID} {
		messages, err := mod.MostRecentChatMessages(did)
		require.NoError(t, err)
		require.Len(t, messages, 2, "a second sweep must not duplicate anything")
	}
}

// TestSweepPrioritizesOwnDIDs: the node's own repos hold what it serves, so
// they go first. Ordering is checked directly because staging a node's own
// did:web account inside the dev environment proves nothing about the order.
func TestSweepPrioritizesOwnDIDs(t *testing.T) {
	dids := []string{"did:plc:a", "did:web:server.example", "did:plc:b", "did:web:broadcaster.example", "did:plc:c"}

	require.Equal(t,
		[]string{"did:web:server.example", "did:web:broadcaster.example", "did:plc:a", "did:plc:b", "did:plc:c"},
		prioritizeDIDs(dids, "did:web:server.example", "did:web:broadcaster.example"))

	// A node whose server and broadcaster are the same host lists it once.
	require.Equal(t,
		[]string{"did:web:server.example", "did:plc:a", "did:plc:b", "did:web:broadcaster.example", "did:plc:c"},
		prioritizeDIDs(dids, "did:web:server.example", "did:web:server.example"))

	// DIDs that are not in the sweep, or not configured, change nothing.
	require.Equal(t, dids, prioritizeDIDs(dids, "did:web:nowhere.example", ""))
	require.Equal(t, dids, prioritizeDIDs(dids))
	require.Nil(t, prioritizeDIDs(nil, "did:web:server.example"))
}

// TestSweepHostLanes: the bucketing a sweep's whole throughput rests on. Repos
// are grouped by PDS host, however the host was written down, and a row with no
// host does not queue up behind the other rows that have none.
func TestSweepHostLanes(t *testing.T) {
	// A PDS is a host however its URL was written down.
	require.Equal(t, "pds.example", sweepLane("did:plc:a", "https://pds.example"))
	require.Equal(t, "pds.example", sweepLane("did:plc:a", "https://PDS.Example/"))
	// A row that does not name one gets a lane of its own, keyed by DID so it
	// can never collide with a host.
	require.Equal(t, "did:did:plc:a", sweepLane("did:plc:a", ""))
	require.Equal(t, "did:did:plc:a", sweepLane("did:plc:a", "   "))

	items := []sweepItem{
		{DID: "own", Lane: sweepLane("own", "https://own.example")},
		{DID: "a1", Lane: sweepLane("a1", "https://a.example")},
		{DID: "b1", Lane: sweepLane("b1", "https://b.example")},
		{DID: "a2", Lane: sweepLane("a2", "https://a.example")},
		{DID: "u1", Lane: sweepLane("u1", "")},
		{DID: "a3", Lane: sweepLane("a3", "https://A.EXAMPLE/")},
		{DID: "u2", Lane: sweepLane("u2", "")},
	}
	// own.example, a.example (three repos, one lane), b.example, and one lane
	// each for the two rows that name no host.
	require.Equal(t, 5, laneCount(items))
	require.Equal(t, 0, laneCount(nil))
}

// TestSweepResolvesUnknownHosts: the sweep's DID list and the PDS column live in
// different databases, so a node with a fresh index knows which repos to sync and
// nothing about where they live. Those repos have to be placed before the sharding
// means anything -- a lane each would be the flat worker pool all over again.
func TestSweepResolvesUnknownHosts(t *testing.T) {
	dir := identity.NewMockDirectory()
	insert := func(did, pds string) {
		dir.Insert(identity.Identity{
			DID:      syntax.DID(did),
			Handle:   syntax.HandleInvalid,
			Services: map[string]identity.ServiceEndpoint{"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: pds}},
		})
	}
	insert("did:plc:one", "https://shared.example")
	insert("did:plc:two", "https://shared.example/")
	insert("did:plc:three", "https://elsewhere.example")
	// Pre-set so resolveIdent never reaches for a real directory.
	atsync := &ATProtoSynchronizer{PLCDirectory: &dir, CachedPLCDirectory: &dir}

	var mu sync.Mutex
	lanes := map[string]string{}
	atsync.feedUnresolved(context.Background(), []sweepItem{
		{DID: "did:plc:one"},
		{DID: "did:plc:two"},
		{DID: "did:plc:missing"}, // no DID document: nothing to place it by
		{DID: "did:plc:three"},
	}, func(item sweepItem) {
		mu.Lock()
		lanes[item.DID] = item.Lane
		mu.Unlock()
	})

	require.Equal(t, map[string]string{
		"did:plc:one":     "shared.example", // two resolving to one host share a lane
		"did:plc:two":     "shared.example",
		"did:plc:missing": "did:did:plc:missing", // unplaceable: its own lane, not a queue
		"did:plc:three":   "elsewhere.example",
	}, lanes)

	// Nothing to do is the normal case, and it must not add anything.
	atsync.feedUnresolved(context.Background(), nil, func(sweepItem) {
		t.Error("add called with no items to resolve")
	})
}

// TestLaneSchedulerStreams is the property the scheduler exists for: work on
// lanes that are already known starts while more items are still arriving,
// without ever breaking one-worker-per-lane or the global cap.
func TestLaneSchedulerStreams(t *testing.T) {
	var mu sync.Mutex
	inflight := map[string]int{}
	maxTotal := 0
	var order []string
	release := make(chan struct{})
	firstStarted := make(chan struct{})
	var once sync.Once

	sched := newLaneScheduler(context.Background(), 2, func(_ context.Context, step sweepStep) bool {
		once.Do(func() { close(firstStarted) })
		mu.Lock()
		inflight[step.Lane]++
		require.LessOrEqual(t, inflight[step.Lane], 1, "two workers on lane %s", step.Lane)
		total := 0
		for _, n := range inflight {
			total += n
		}
		if total > maxTotal {
			maxTotal = total
		}
		order = append(order, step.DID)
		mu.Unlock()
		<-release
		mu.Lock()
		inflight[step.Lane]--
		mu.Unlock()
		return false
	})

	sched.add(sweepItem{DID: "a1", Lane: "hostA"})
	// The first item is being worked before the rest have even been added --
	// that is the streaming property.
	<-firstStarted
	sched.add(sweepItem{DID: "a2", Lane: "hostA"})
	sched.add(sweepItem{DID: "b1", Lane: "hostB"})
	sched.add(sweepItem{DID: "c1", Lane: "hostC"})
	close(release)

	lanes, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, 3, lanes)
	require.ElementsMatch(t, []string{"a1", "a2", "b1", "c1"}, order)
	require.LessOrEqual(t, maxTotal, 2, "global lane cap exceeded")
	require.Less(t, indexOf(order, "a1"), indexOf(order, "a2"), "lane order must be FIFO")
}

// TestLaneSchedulerCancelled: a dead context stops a scheduler without working
// anything more and without hanging wait.
func TestLaneSchedulerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sched := newLaneScheduler(ctx, 2, func(context.Context, sweepStep) bool {
		t.Error("work ran under a cancelled context")
		return false
	})
	sched.add(sweepItem{DID: "a1", Lane: "hostA"})
	_, err := sched.wait()
	require.ErrorIs(t, err, context.Canceled)
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// stepLabel renders a step the way the lane-program tests compare them: which
// repo, and whether it is the shallow sync or the nth window.
func stepLabel(step sweepStep) string {
	if step.Check {
		return step.DID + "/check"
	}
	if !step.Deepen {
		return step.DID + "/shallow"
	}
	return fmt.Sprintf("%s/window%d", step.DID, step.Windows+1)
}

// TestSweepLaneProgramChecksFirst: a lane's head checks come before its other
// work, because each is one request that says whether the rest of the work on
// that repo is the right work -- a repo whose recent records are wrong is not
// made righter by deepening it. A check that finds drift adds a shallow sync
// the sweep did not know it had, and that sync still preempts the ladder.
func TestSweepLaneProgramChecksFirst(t *testing.T) {
	ready := make(chan struct{})
	var mu sync.Mutex
	var steps []string
	var sched *laneScheduler
	windows := map[string]int{}

	sched = newLaneScheduler(context.Background(), 4, func(_ context.Context, step sweepStep) bool {
		<-ready
		mu.Lock()
		defer mu.Unlock()
		steps = append(steps, stepLabel(step))
		switch {
		case step.Check:
			if step.DID == "drifted" {
				// What sweepCheck does with drift: hand the repair back to
				// this same lane, where it goes ahead of the ladder.
				sched.add(sweepItem{DID: step.DID, Lane: step.Lane})
				return false
			}
			return true // current, and still owes history
		case !step.Deepen:
			return true
		default:
			windows[step.DID]++
			return false
		}
	})
	sched.add(sweepItem{DID: "current", Lane: "pds.example", Check: true})
	sched.add(sweepItem{DID: "drifted", Lane: "pds.example", Check: true})
	sched.add(sweepItem{DID: "new", Lane: "pds.example"})
	close(ready)
	_, err := sched.wait()
	require.NoError(t, err)

	require.Equal(t, []string{
		"current/check", "drifted/check",
		"new/shallow", "drifted/shallow",
		"current/window1", "new/window1", "drifted/window1",
	}, steps)
}

// TestSweepLaneProgramShallowFirst: a host's repos are all made servable before
// any of them is deepened, and each repo's ladder starts at the bottom rung. A
// sweep that deepened one repo's history while another on the same host had
// never been read at all would be optimizing the wrong thing.
func TestSweepLaneProgramShallowFirst(t *testing.T) {
	// Nothing runs until every repo is queued, so that this is a statement
	// about the program and not about who won a race to be added.
	ready := make(chan struct{})
	var mu sync.Mutex
	var steps []string
	windows := map[string]int{}

	sched := newLaneScheduler(context.Background(), 4, func(_ context.Context, step sweepStep) bool {
		<-ready
		mu.Lock()
		defer mu.Unlock()
		steps = append(steps, stepLabel(step))
		if !step.Deepen {
			return true
		}
		windows[step.DID]++
		return windows[step.DID] < 2
	})
	for _, did := range []string{"a", "b", "c"} {
		sched.add(sweepItem{DID: did, Lane: "pds.example"})
	}
	close(ready)
	lanes, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, 1, lanes)

	require.Equal(t, []string{
		"a/shallow", "b/shallow", "c/shallow",
		"a/window1", "b/window1", "c/window1",
		"a/window2", "b/window2", "c/window2",
	}, steps)
}

// TestSweepLaneProgramBreadthFirst is the guarantee the global rounds used to
// buy, rescoped to one host: no repo gets its (n+1)th window while another repo
// on the same host is still waiting for its nth. That is what puts the same
// horizon behind every account a PDS serves, and it is now free -- a lane
// reaching it does not make any other lane wait.
func TestSweepLaneProgramBreadthFirst(t *testing.T) {
	want := map[string]int{"a": 2, "b": 5, "c": 3, "d": 5}
	ready := make(chan struct{})
	var mu sync.Mutex
	windows := map[string]int{}
	pending := map[string]bool{}
	for did := range want {
		pending[did] = true
	}

	sched := newLaneScheduler(context.Background(), 4, func(_ context.Context, step sweepStep) bool {
		<-ready
		mu.Lock()
		defer mu.Unlock()
		require.True(t, step.Deepen, "these repos are already servable")
		require.Equal(t, windows[step.DID], step.Windows,
			"%s: a step knows how many windows its repo has had", step.DID)
		for did := range pending {
			require.LessOrEqual(t, step.Windows, windows[did],
				"%s took window %d while %s was still waiting for window %d",
				step.DID, step.Windows+1, did, windows[did]+1)
		}
		windows[step.DID]++
		if windows[step.DID] >= want[step.DID] {
			delete(pending, step.DID)
			return false
		}
		return true
	})
	for _, did := range []string{"a", "b", "c", "d"} {
		sched.add(sweepItem{DID: did, Lane: "pds.example", Deepen: true})
	}
	close(ready)
	_, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, want, windows, "every repo got exactly the ladder it asked for")
}

// TestSweepLaneProgramLateShallowPreempts: a repo whose host is resolved after
// its lane started work joins that lane mid-ladder, and is synced before the
// lane takes another rung -- an account nobody has read yet is worth more than
// another month of history for accounts that are already being served. It then
// joins the ladder at the bottom, so the breadth-first order absorbs it instead
// of leaving it a lap behind.
func TestSweepLaneProgramLateShallowPreempts(t *testing.T) {
	var mu sync.Mutex
	var steps []string
	windows := map[string]int{}
	var once sync.Once
	var sched *laneScheduler

	sched = newLaneScheduler(context.Background(), 4, func(_ context.Context, step sweepStep) bool {
		mu.Lock()
		steps = append(steps, stepLabel(step))
		if step.Deepen {
			windows[step.DID]++
		}
		n := windows[step.DID]
		mu.Unlock()

		if !step.Deepen {
			return true // a fresh sync always leaves history to fetch here
		}
		if step.DID == "a" && n == 2 {
			// The resolver finally placed a repo on this host, half way
			// through the ladder the lane was already running.
			once.Do(func() { sched.add(sweepItem{DID: "late", Lane: "pds.example"}) })
		}
		if step.DID == "late" {
			return n < 2
		}
		return n < 4
	})
	sched.add(sweepItem{DID: "a", Lane: "pds.example", Deepen: true})
	sched.add(sweepItem{DID: "b", Lane: "pds.example", Deepen: true})
	_, err := sched.wait()
	require.NoError(t, err)

	require.Equal(t, []string{
		"a/window1", "b/window1", "a/window2",
		"late/shallow", // straight away, ahead of b's second window
		"late/window1", // and its first rung before anyone's third
		"b/window2", "late/window2",
		"a/window3", "b/window3",
		"a/window4", "b/window4",
	}, steps)
}

// TestSweepLaneProgramsAreIndependent is the whole point of this design: a host
// that is not answering cannot hold up a host that is. Measured on a 20k-repo
// sweep, the global phase and round barriers spent roughly half the wall clock
// with most lanes idle behind stragglers exactly like this one.
func TestSweepLaneProgramsAreIndependent(t *testing.T) {
	hold := make(chan struct{})
	finished := make(chan struct{})
	var mu sync.Mutex
	var fast []string
	windows := map[string]int{}
	const fastSteps = 8 // two repos, each a shallow sync and three windows

	sched := newLaneScheduler(context.Background(), 4, func(_ context.Context, step sweepStep) bool {
		if step.Lane == "stuck.example" {
			<-hold
			return false
		}
		mu.Lock()
		defer mu.Unlock()
		fast = append(fast, stepLabel(step))
		if len(fast) == fastSteps {
			close(finished)
		}
		if !step.Deepen {
			return true
		}
		windows[step.DID]++
		return windows[step.DID] < 3
	})
	// The stuck host goes first, so it also holds the first slot: priority
	// order must not become priority blocking.
	sched.add(sweepItem{DID: "stuck1", Lane: "stuck.example"})
	sched.add(sweepItem{DID: "a", Lane: "pds.example"})
	sched.add(sweepItem{DID: "b", Lane: "pds.example"})

	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("the working host's lane never finished while another host was stuck")
	}
	mu.Lock()
	require.Equal(t, []string{
		"a/shallow", "b/shallow",
		"a/window1", "b/window1",
		"a/window2", "b/window2",
		"a/window3", "b/window3",
	}, fast, "a whole per-host program ran to the end with another host mid-sync")
	mu.Unlock()

	close(hold)
	lanes, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, 2, lanes)
}

// TestSweepLaneProgramSpinGuard: a repo that never admits to being finished
// still costs a bounded number of windows per sweep.
func TestSweepLaneProgramSpinGuard(t *testing.T) {
	var mu sync.Mutex
	steps := 0
	sched := newLaneScheduler(context.Background(), 2, func(_ context.Context, step sweepStep) bool {
		mu.Lock()
		defer mu.Unlock()
		steps++
		require.LessOrEqual(t, step.Windows, maxDeepenRounds)
		return true
	})
	sched.add(sweepItem{DID: "a", Lane: "pds.example"})
	_, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, maxDeepenRounds+1, steps, "one shallow sync and a bounded ladder")
}

// TestSweepLanesNeverShareAHost is the property the lanes exist for: a sweep
// never has two workers on one PDS at the same time, however many workers it is
// allowed. Nothing else in a sweep is worth optimizing until that holds -- walks
// that share a host interleave their chunk fetches and run four to ten times
// slower.
func TestSweepLanesNeverShareAHost(t *testing.T) {
	const cap = 3
	var items []sweepItem
	for i := 0; i < 20; i++ {
		host := fmt.Sprintf("pds%d.example", i%4)
		items = append(items, sweepItem{DID: fmt.Sprintf("did:plc:%d", i), Lane: sweepLane("", "https://"+host)})
	}

	var mu sync.Mutex
	active := map[string]string{} // lane -> the DID holding it
	var order []string
	windows := map[string]int{}
	inFlight, maxInFlight := 0, 0
	sched := newLaneScheduler(context.Background(), cap, func(_ context.Context, step sweepStep) bool {
		mu.Lock()
		holder, busy := active[step.Lane]
		require.False(t, busy, "%s and %s ran on %s at once", step.DID, holder, step.Lane)
		active[step.Lane] = step.DID
		inFlight++
		maxInFlight = max(maxInFlight, inFlight)
		mu.Unlock()

		// Long enough that a broken limiter or a shared lane would overlap here,
		// short enough to be free.
		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		delete(active, step.Lane)
		inFlight--
		order = append(order, step.DID)
		if !step.Deepen {
			return true
		}
		windows[step.DID]++
		return windows[step.DID] < 2
	})
	for _, item := range items {
		sched.add(item)
	}
	lanes, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, 4, lanes, "four hosts, and a cap of three: some lane waited for a slot")
	require.Len(t, order, 3*len(items), "every repo got its sync and both its windows")
	require.LessOrEqual(t, maxInFlight, cap, "the cap bounds lanes in flight")
	require.Greater(t, maxInFlight, 1, "and lanes really do run in parallel")
}

// TestSweepLanesRunOwnDIDsFirst: the node's own repos hold what it serves, so
// their lane is the first one given a slot -- the priority order prioritizeDIDs
// produces has to survive the bucketing, which means slots go out in the order
// lanes were added rather than in whatever order their goroutines woke up.
func TestSweepLanesRunOwnDIDsFirst(t *testing.T) {
	dids := prioritizeDIDs([]string{"did:plc:a", "did:web:server.example", "did:plc:b"}, "did:web:server.example")

	var mu sync.Mutex
	var order []string
	// One slot, and every repo on its own host, so lane order is the only
	// thing deciding.
	sched := newLaneScheduler(context.Background(), 1, func(_ context.Context, step sweepStep) bool {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step.DID)
		return false
	})
	for _, did := range dids {
		sched.add(sweepItem{DID: did, Lane: sweepLane(did, "https://"+did+".pds.example")})
	}
	_, err := sched.wait()
	require.NoError(t, err)
	require.Equal(t, []string{"did:web:server.example", "did:plc:a", "did:plc:b"}, order)
}

// TestSweepLanesStopOnCancel: a sweep is cancellable at every point, and a lane
// checks the context between steps rather than at the end of a program that
// would otherwise run for hours.
func TestSweepLanesStopOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	ran := 0
	sched := newLaneScheduler(ctx, 4, func(_ context.Context, step sweepStep) bool {
		mu.Lock()
		defer mu.Unlock()
		ran++
		if ran == 2 {
			cancel()
		}
		// Never finished: only the cancellation can end this lane.
		return true
	})
	for i := 0; i < 40; i++ {
		sched.add(sweepItem{DID: fmt.Sprintf("did:plc:%d", i), Lane: "pds.example"})
	}
	_, err := sched.wait()
	require.ErrorIs(t, err, context.Canceled)
	mu.Lock()
	defer mu.Unlock()
	require.Less(t, ran, 40, "the lane stopped instead of running its program out")
}

// TestSweepConcurrencyFlag: the cap comes from --sweep-concurrency, and an unset
// or nonsense value is the documented default.
// TestSweepLoopRepeats: the boot sweep always runs, and after it the ticker
// keeps running them until the node goes away. That repetition is what makes
// the head check a reconciliation loop rather than a one-off.
func TestSweepLoopRepeats(t *testing.T) {
	atsync := &ATProtoSynchronizer{}

	// A disabled ticker still sweeps once at boot.
	var once atomic.Int64
	atsync.sweepLoop(context.Background(), 0, func(context.Context) { once.Add(1) })
	require.Equal(t, int64(1), once.Load())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runs := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		atsync.sweepLoop(ctx, time.Millisecond, func(context.Context) {
			select {
			case runs <- struct{}{}:
			default:
			}
		})
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-runs:
		case <-time.After(10 * time.Second):
			t.Fatalf("only %d sweeps ran", i)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the sweep loop outlived its context")
	}
}

// TestSweepOnceSkipsWhileRunning: a sweep of a large index can take longer than
// the interval, and two at once would double every host's request rate to do
// the same work twice. The tick is dropped, not queued.
func TestSweepOnceSkipsWhileRunning(t *testing.T) {
	atsync := &ATProtoSynchronizer{}
	ctx := context.Background()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		atsync.sweepOnce(ctx, func(context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	var skipped atomic.Bool
	skipped.Store(true)
	atsync.sweepOnce(ctx, func(context.Context) error {
		skipped.Store(false)
		return nil
	})
	require.True(t, skipped.Load(), "a second sweep must not start on top of the first")

	close(release)
	<-finished

	// And the slot is handed back, so the next tick sweeps.
	var ran atomic.Bool
	atsync.sweepOnce(ctx, func(context.Context) error {
		ran.Store(true)
		return fmt.Errorf("a sweep that fails is logged, not fatal")
	})
	require.True(t, ran.Load())
}

// TestSweepIntervalConfig: how often a node re-checks the repos it indexes.
func TestSweepIntervalConfig(t *testing.T) {
	require.Equal(t, config.DefaultSweepInterval, (&ATProtoSynchronizer{}).sweepInterval(),
		"a synchronizer without a CLI still re-sweeps")
	require.Equal(t, 90*time.Minute,
		(&ATProtoSynchronizer{CLI: &config.CLI{SweepInterval: 90 * time.Minute}}).sweepInterval())
	require.Equal(t, time.Duration(0),
		(&ATProtoSynchronizer{CLI: &config.CLI{SweepInterval: 0}}).sweepInterval(), "0 disables the ticker")
}

func TestSweepConcurrencyFlag(t *testing.T) {
	require.Equal(t, config.DefaultSweepConcurrency, (&ATProtoSynchronizer{}).sweepConcurrency(),
		"a synchronizer without a CLI still sweeps")
	require.Equal(t, config.DefaultSweepConcurrency,
		(&ATProtoSynchronizer{CLI: &config.CLI{}}).sweepConcurrency(), "unset means the default")
	require.Equal(t, config.DefaultSweepConcurrency,
		(&ATProtoSynchronizer{CLI: &config.CLI{SweepConcurrency: -1}}).sweepConcurrency())
	require.Equal(t, 64,
		(&ATProtoSynchronizer{CLI: &config.CLI{SweepConcurrency: 64}}).sweepConcurrency())
}

// TestSweepProgressStatusLine covers the one line an operator watches. There
// are no phases left to name -- every host runs its own program -- so the line
// is two fractions, the windows they took, and the horizon in unix seconds:
//
//	backfill sweep shallow=19000/20747 deepened=4300/20013 windows=41022 horizon=1753142400
func TestSweepProgressStatusLine(t *testing.T) {
	var progress sweepProgress

	// Before anything starts there is nothing to say.
	require.Equal(t,
		[]any{"shallow", "0/0", "deepened", "0/0", "windows", 0, "horizon", int64(0)},
		progress.status())

	day := time.Now().Add(-InitialWindow)
	week := time.Now().Add(-7 * 24 * time.Hour)
	month := time.Now().Add(-30 * 24 * time.Hour)

	// Three repos to make servable, one already servable and mid-ladder: the
	// horizon is that one's watermark. Nothing to head-check, so the line does
	// not mention checking -- which is a fresh node's first sweep exactly.
	progress.begin(3, 0, map[string]time.Time{"did:plc:old": week})
	require.Equal(t,
		[]any{"shallow", "0/3", "deepened", "0/1", "windows", 0, "horizon", week.Unix()},
		progress.status())

	// A repo that has just been synced is servable, and joins the ladder: the
	// denominator grows as the sweep discovers who needs deepening, and the
	// horizon follows the least-deepened repo.
	progress.synced()
	progress.laddered("did:plc:new", day)
	require.Equal(t,
		[]any{"shallow", "1/3", "deepened", "0/2", "windows", 0, "horizon", day.Unix()},
		progress.status())

	// Windows count as they land, and each moves one repo's watermark. The
	// horizon only moves when the laggard does.
	progress.window("did:plc:new", week)
	require.Equal(t,
		[]any{"shallow", "1/3", "deepened", "0/2", "windows", 1, "horizon", week.Unix()},
		progress.status())
	progress.window("did:plc:old", month)
	require.Equal(t,
		[]any{"shallow", "1/3", "deepened", "0/2", "windows", 2, "horizon", week.Unix()},
		progress.status())

	// A repo with its whole history stops holding the horizon back, and when
	// nothing is left to deepen there is no horizon at all.
	progress.window("did:plc:new", month)
	progress.deepened("did:plc:new")
	require.Equal(t,
		[]any{"shallow", "1/3", "deepened", "1/2", "windows", 3, "horizon", month.Unix()},
		progress.status())
	progress.deepened("did:plc:old")
	require.Equal(t,
		[]any{"shallow", "1/3", "deepened", "2/2", "windows", 3, "horizon", int64(0)},
		progress.status())

	// The ticker stops when told to, without leaking a goroutine.
	stop := progress.start(context.Background())
	stop()

	// A warm node's sweep starts with a head check per servable repo, and says
	// so until it has made all of them. A check that finds drift is a shallow
	// sync this sweep did not know it had, so the denominator grows.
	var warm sweepProgress
	warm.begin(1, 2, nil)
	require.Equal(t,
		[]any{"checked", "0/2", "shallow", "0/1", "deepened", "0/0", "windows", 0, "horizon", int64(0)},
		warm.status())
	warm.checked()
	warm.checked()
	warm.repairing()
	require.Equal(t,
		[]any{"checked", "2/2", "shallow", "0/2", "deepened", "0/0", "windows", 0, "horizon", int64(0)},
		warm.status())
}

// walkAll walks a repo's ranges against the dev PDS and returns the paths, so
// tests can wait for the PDS to have committed what they wrote.
func walkAll(ctx context.Context, dev *devenv.DevEnv, did string, ranges []reposync.KeyRange) ([]string, error) {
	xrpcc := &xrpc.Client{Host: dev.PDSURL, Client: &aqhttp.Client}
	fetcher := &reposync.CachedFetcher{
		Cache: reposync.NewMemoryBlockCache(),
		Inner: &reposync.XRPCBlockFetcher{Client: xrpcc, DID: did},
	}
	head, err := reposync.FetchVerifiedHead(ctx, xrpcc, fetcher, dev.TestDirectory(), did)
	if err != nil {
		return nil, err
	}
	var paths []string
	err = (&reposync.Walker{Fetcher: fetcher}).WalkRanges(ctx, head.Root, ranges,
		func(path string, _ cid.Cid, _ []byte) error {
			paths = append(paths, path)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// watchBus counts what a topic publishes, for asserting that re-walked records
// do not reach subscribers twice.
func watchBus(t *testing.T, b *bus.Bus, topic string) func() int {
	t.Helper()
	ch := b.Subscribe(topic)
	t.Cleanup(func() { b.Unsubscribe(topic, ch) })
	var mu sync.Mutex
	count := 0
	go func() {
		for range ch {
			mu.Lock()
			count++
			mu.Unlock()
		}
	}()
	return func() int {
		// The publish is asynchronous; give it a moment to happen before
		// reporting a count that a test is about to assert on.
		time.Sleep(250 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return count
	}
}
