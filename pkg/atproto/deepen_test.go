package atproto

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/reposync"
)

// TestDeepenForeverTrickles is the deepener end to end: nobody tells it what to
// work on. It scans the index for repos that are servable but still owe
// history, ladders them down to genesis, and keeps running afterwards -- until
// its context ends, which it must actually notice.
func TestDeepenForeverTrickles(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	atsync, mod := backfillTestSynchronizer(t, dev)
	// Unlimited, which is what an operator sets to rush an initial build. The
	// pace is production's business; a test that waited on the real one would
	// wait a second a window to prove nothing.
	atsync.CLI.DeepenRate = 0

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	// Old enough to be several rungs down the ladder, so finishing this repo
	// takes a real walk rather than one window.
	createBackfillRecord(t, user, "place.stream.chat.message",
		reposync.TIDForTime(time.Now().Add(-200*24*time.Hour)),
		chatMessageRecord(user.DID, "from the deep window"))

	_, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	synced, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, synced.Version, "the shallow sync made the repo servable")
	require.False(t, synced.BackfillDone, "and left the old message for the deepener")

	done := make(chan struct{})
	go func() {
		defer close(done)
		atsync.DeepenForever(ctx)
	}()

	require.NoError(t, untilNoErrors(t, func() error {
		repo, err := mod.GetRepo(user.DID)
		if err != nil {
			return err
		}
		if !repo.BackfillDone {
			return fmt.Errorf("repo is still deepening at %q", repo.BackfillFloor)
		}
		return nil
	}), "the deepener should find the repo by scanning and walk it to the end")

	messages, err := mod.MostRecentChatMessages(user.DID)
	require.NoError(t, err)
	require.Len(t, messages, 1, "the message in the deepest window was indexed")

	// With nothing left to deepen the loop is idling; cancelling must end it
	// rather than leave it waiting out its idle interval.
	cancel()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DeepenForever outlived its context")
	}
}

// TestSweepForeverLeavesLaddersToTheDeepener is the split itself. The pass a
// serving node runs on a timer reconciles and stops there -- it must not spend
// hours walking history while the head checks queue up behind it -- and the
// full sweep `streamplace sync` runs still finishes the job.
func TestSweepForeverLeavesLaddersToTheDeepener(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	createBackfillRecord(t, user, "place.stream.chat.message",
		reposync.TIDForTime(time.Now().Add(-72*time.Hour)),
		chatMessageRecord(user.DID, "from the deep window"))

	_, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	synced, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.False(t, synced.BackfillDone, "there is history left to fetch")

	// The pass SweepForever runs: head checks and repairs, no ladders.
	require.NoError(t, atsync.sweep(ctx, false))
	checked, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.False(t, checked.BackfillDone, "a reconciliation pass does not deepen")
	require.Equal(t, synced.BackfillFloor, checked.BackfillFloor,
		"and does not walk so much as one window")
	require.NotEmpty(t, checked.Version, "while still leaving the repo servable")

	// And the full sweep is untouched: one command, run to completion.
	require.NoError(t, atsync.Sweep(ctx))
	deepened, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.True(t, deepened.BackfillDone, "a full sweep still finishes the ladder")
}

// TestDeepenRateLimiter: windows a minute, turned into the limiter that paces
// them. The unit is the window walk -- one range walk against one host, one
// burst of writes into the index -- so this is the whole of what --deepen-rate
// means.
func TestDeepenRateLimiter(t *testing.T) {
	// The default: one window a second, with a few seconds of slack so a lane
	// coming back from a slow host is not immediately made to wait on a clock.
	def := deepenLimiter(config.DefaultDeepenRate)
	require.Equal(t, rate.Limit(1), def.Limit())
	require.Equal(t, 3, def.Burst())

	fast := deepenLimiter(600)
	require.Equal(t, rate.Limit(10), fast.Limit())
	require.Equal(t, 30, fast.Burst())

	// A trickle slower than the burst window still hands out whole windows.
	slow := deepenLimiter(6)
	require.InDelta(t, 0.1, float64(slow.Limit()), 1e-9)
	require.Equal(t, 1, slow.Burst())

	// 0 is no cap at all, expressed as a limiter that never waits, so there is
	// nothing to nil-check where the windows are walked.
	require.Equal(t, rate.Inf, deepenLimiter(0).Limit())
	require.True(t, deepenLimiter(0).Allow())
	require.True(t, deepenLimiter(0).AllowN(time.Now(), 1000))

	require.Equal(t, "unlimited", deepenRateLabel(deepenLimiter(0)))
	require.Equal(t, "60/min", deepenRateLabel(def))
	require.Equal(t, "600/min", deepenRateLabel(fast))
}

// TestDeepenRateFlag: where that number comes from, including the two values
// that do not mean what a repo count would.
func TestDeepenRateFlag(t *testing.T) {
	require.Equal(t, config.DefaultDeepenRate, (&ATProtoSynchronizer{}).deepenRate(),
		"a synchronizer without a CLI still paces itself")
	require.Equal(t, config.DefaultDeepenRate,
		(&ATProtoSynchronizer{CLI: &config.CLI{DeepenRate: -1}}).deepenRate(), "negative is the default")
	require.Equal(t, 0,
		(&ATProtoSynchronizer{CLI: &config.CLI{DeepenRate: 0}}).deepenRate(), "0 is unlimited, not unset")
	require.Equal(t, 12,
		(&ATProtoSynchronizer{CLI: &config.CLI{DeepenRate: 12}}).deepenRate())
}

// TestSweepProgressLadderlessStatusLines: a pass says nothing about counters it
// does not run. A reconciliation sweep reporting deepened=0/0 for the rest of
// the node's life would be a lie about work that is happening somewhere else,
// and the deepener has no shallow syncs to report at all.
func TestSweepProgressLadderlessStatusLines(t *testing.T) {
	check := &sweepProgress{mode: sweepModeCheck}
	check.begin(1, 2, nil)
	check.checked()
	require.Equal(t, []any{"checked", "1/2", "shallow", "0/1"}, check.status())
	require.Equal(t, "backfill sweep", check.mode.line())

	week := time.Now().Add(-7 * 24 * time.Hour)
	deep := &sweepProgress{mode: sweepModeDeepen, rate: "60/min"}
	deep.begin(0, 0, map[string]time.Time{"did:plc:old": week})
	deep.window("did:plc:old", week)
	require.Equal(t,
		[]any{"repos", "0/1", "windows", 1, "horizon", week.Unix(), "rate", "60/min"},
		deep.status())

	deep.deepened("did:plc:old")
	require.Equal(t,
		[]any{"repos", "1/1", "windows", 1, "horizon", int64(0), "rate", "60/min"},
		deep.status())
	require.Equal(t, "deepening history", deep.mode.line())
	require.Equal(t, 1, deep.windowsWalked())
}
