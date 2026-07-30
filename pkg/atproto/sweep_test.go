package atproto

import (
	"context"
	"fmt"
	"sync"
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
	for rung, want := range wantAfterRung {
		require.False(t, done, "the ladder finished early at rung %d", rung)
		done, err = atsync.DeepenRepo(ctx, user.DID)
		require.NoError(t, err, "rung %d", rung)
		require.Equal(t, want, countMessages(), "message count after rung %d", rung)
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
	again, err := atsync.DeepenRepo(ctx, user.DID)
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
// are grouped by PDS host, in the order they arrive, so the lane list starts
// with the lane holding whatever prioritizeDIDs put first.
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
	lanes := hostLanes(items)

	require.Equal(t, [][]string{
		{"own"},            // the priority DID's host, first because it was first
		{"a1", "a2", "a3"}, // one lane per host, whatever the URL looked like
		{"b1"},
		{"u1"}, // and unknown-PDS rows do not queue up behind each other
		{"u2"},
	}, laneDIDs(lanes))

	require.Empty(t, hostLanes(nil))
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

	sched := newLaneScheduler(context.Background(), 2, func(_ context.Context, item sweepItem) {
		once.Do(func() { close(firstStarted) })
		mu.Lock()
		inflight[item.Lane]++
		require.LessOrEqual(t, inflight[item.Lane], 1, "two workers on lane %s", item.Lane)
		total := 0
		for _, n := range inflight {
			total += n
		}
		if total > maxTotal {
			maxTotal = total
		}
		order = append(order, item.DID)
		mu.Unlock()
		<-release
		mu.Lock()
		inflight[item.Lane]--
		mu.Unlock()
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
	sched := newLaneScheduler(ctx, 2, func(context.Context, sweepItem) {
		t.Error("work ran under a cancelled context")
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
	inFlight, maxInFlight := 0, 0
	err := runLanes(context.Background(), cap, hostLanes(items), func(ctx context.Context, item sweepItem) {
		mu.Lock()
		holder, busy := active[item.Lane]
		require.False(t, busy, "%s and %s ran on %s at once", item.DID, holder, item.Lane)
		active[item.Lane] = item.DID
		inFlight++
		maxInFlight = max(maxInFlight, inFlight)
		mu.Unlock()

		// Long enough that a broken limiter or a shared lane would overlap here,
		// short enough to be free.
		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		delete(active, item.Lane)
		inFlight--
		order = append(order, item.DID)
		mu.Unlock()
	})
	require.NoError(t, err)
	require.Len(t, order, len(items), "every repo ran exactly once")
	require.LessOrEqual(t, maxInFlight, cap, "the cap bounds lanes in flight")
	require.Greater(t, maxInFlight, 1, "and lanes really do run in parallel")

	// Four hosts, cap of three: at least one lane waited for a slot, which is
	// the case that has to not deadlock.
	require.Equal(t, 4, len(hostLanes(items)))
}

// TestSweepLanesRunOwnDIDsFirst: the node's own repos hold what it serves, so
// their lane is the first one scheduled -- the priority order prioritizeDIDs
// produces has to survive the bucketing.
func TestSweepLanesRunOwnDIDsFirst(t *testing.T) {
	dids := prioritizeDIDs([]string{"did:plc:a", "did:web:server.example", "did:plc:b"}, "did:web:server.example")
	items := make([]sweepItem, 0, len(dids))
	for _, did := range dids {
		// Every repo on its own host, so lane order is the only thing deciding.
		items = append(items, sweepItem{DID: did, Lane: sweepLane(did, "https://"+did+".pds.example")})
	}

	var mu sync.Mutex
	var order []string
	// One slot: lanes are started in order, so the first thing that runs is the
	// first lane.
	require.NoError(t, runLanes(context.Background(), 1, hostLanes(items), func(ctx context.Context, item sweepItem) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, item.DID)
	}))
	require.Equal(t, []string{"did:web:server.example", "did:plc:a", "did:plc:b"}, order)
}

// TestSweepLanesStopOnCancel: a sweep is cancellable at every point, and a lane
// checks the context between repos rather than after all of them.
func TestSweepLanesStopOnCancel(t *testing.T) {
	items := make([]sweepItem, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, sweepItem{DID: fmt.Sprintf("did:plc:%d", i), Lane: "pds.example"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	ran := 0
	err := runLanes(ctx, 4, hostLanes(items), func(ctx context.Context, item sweepItem) {
		mu.Lock()
		ran++
		if ran == 2 {
			cancel()
		}
		mu.Unlock()
	})
	require.ErrorIs(t, err, context.Canceled)
	mu.Lock()
	defer mu.Unlock()
	require.Less(t, ran, len(items), "the run stopped instead of draining the lane")
}

// TestSweepConcurrencyFlag: the cap comes from --sweep-concurrency, and an unset
// or nonsense value is the documented default.
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

// laneDIDs renders lanes for comparison.
func laneDIDs(lanes [][]sweepItem) [][]string {
	out := make([][]string, 0, len(lanes))
	for _, lane := range lanes {
		dids := make([]string, 0, len(lane))
		for _, item := range lane {
			dids = append(dids, item.DID)
		}
		out = append(out, dids)
	}
	return out
}

// TestSweepProgressStatusLine covers the one line an operator watches: it names
// the phase, counts finished repos against the total, and reports the horizon
// as unix seconds.
func TestSweepProgressStatusLine(t *testing.T) {
	var progress sweepProgress

	// Before anything starts there is nothing to say.
	require.Equal(t, []any{"phase", "", "users", 0, "total", 0, "horizon", int64(0)}, progress.status())

	horizon := time.Now().Add(-InitialWindow)
	progress.begin(sweepPhaseShallow, 3, horizon)
	progress.finished()
	require.Equal(t,
		[]any{"phase", "shallow", "users", 1, "total", 3, "horizon", horizon.Unix()},
		progress.status())

	// A new phase resets the count and moves the horizon.
	deeper := time.Now().Add(-30 * 24 * time.Hour)
	progress.begin(sweepPhaseDeepen, 2, deeper)
	require.Equal(t,
		[]any{"phase", "deepen", "users", 0, "total", 2, "horizon", deeper.Unix()},
		progress.status())
	progress.finished()
	progress.finished()
	deepest := time.Now().Add(-180 * 24 * time.Hour)
	progress.setHorizon(deepest)
	require.Equal(t,
		[]any{"phase", "deepen", "users", 2, "total", 2, "horizon", deepest.Unix()},
		progress.status())

	// The ticker stops when told to, without leaking a goroutine.
	stop := progress.start(context.Background())
	stop()
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
