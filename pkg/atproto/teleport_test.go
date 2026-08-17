package atproto

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// TestTeleportArrivalNotFromBackfill: a teleport record indexed by a backfill
// is history, and history does not arrive.
//
// The arrival notification is scheduled for ten seconds after the teleport
// starts, and a teleport from last week is already past that, so a fresh index
// reading an account's repo would announce every teleport it has ever done, all
// at once, to the streamers they pointed at. Indexing the record is right;
// announcing it is not.
func TestTeleportArrivalNotFromBackfill(t *testing.T) {
	ctx := context.Background()
	atsync, mod, b := offlineSynchronizer(t)

	traveller := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	streamer := "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     traveller,
		Handle:  "traveller.test",
		PDS:     "http://127.0.0.1:1",
		Version: "3lrev00000000",
	}))

	// Watch the streamer's topic, which is where an arrival is announced.
	ch := b.Subscribe(streamer)
	defer b.Unsubscribe(streamer, ch)
	var mu sync.Mutex
	var arrivals []bus.Message
	go func() {
		for msg := range ch {
			mu.Lock()
			arrivals = append(arrivals, msg)
			mu.Unlock()
		}
	}()
	countArrivals := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(arrivals)
	}

	duration := int64(600)
	index := func(rkey, startsAt string, isFirstSync bool) {
		t.Helper()
		rec := &placestream.LiveTeleport{
			LexiconTypeID:   "place.stream.live.teleport",
			Streamer:        streamer,
			StartsAt:        startsAt,
			DurationSeconds: &duration,
		}
		var buf bytes.Buffer
		require.NoError(t, rec.MarshalCBOR(&buf))
		recCBOR := buf.Bytes()
		rcid, err := spid.GetCID(rec)
		require.NoError(t, err)
		require.NoError(t, atsync.handleCreateUpdate(ctx, traveller, syntax.RecordKey(rkey),
			&recCBOR, rcid.String(), syntax.NSID("place.stream.live.teleport"), false, isFirstSync))
	}

	// A teleport from last week, met during a backfill. The notification is
	// scheduled with no wait at all, so if it were scheduled we would see it.
	past := time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	index("3lteleportold0", past, true)
	time.Sleep(250 * time.Millisecond)
	require.Equal(t, 0, countArrivals(), "a backfilled teleport must not announce an arrival")
	stored, err := mod.GetTeleportByURI("at://" + traveller + "/place.stream.live.teleport/3lteleportold0")
	require.NoError(t, err)
	require.NotNil(t, stored, "the record is still indexed; only the announcement is live-only")

	// The same record arriving live still announces: this is a guard on
	// backfills, not a change to what a teleport does.
	index("3lteleportnew0", past, false)
	require.Eventually(t, func() bool { return countArrivals() == 1 }, 5*time.Second, 10*time.Millisecond,
		"a live teleport still announces an arrival")
}
