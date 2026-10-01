package atproto

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
)

// TestAutoPublishVODScheduledWhenSeenEnding: sync schedules automatic VOD
// publishing when it sees a livestream record end, for a streamer who has it
// on then. An ended record seen again (a redelivery, an edit, one met in a
// backfill) never schedules: turning the setting on later must not publish a
// recording from before.
func TestAutoPublishVODScheduledWhenSeenEnding(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)
	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     did,
		Handle:  "streamer.test",
		PDS:     "http://127.0.0.1:1",
		Version: "3lrev00000000",
	}))
	optIn := func(on bool) {
		t.Helper()
		require.NoError(t, atsync.StatefulDB.PutUserPreferences(ctx, &statedb.UserPreferences{RepoDID: did, AutoPublishVODs: on}))
	}

	// Fixed times, so indexing the same record again is byte-identical.
	createdAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	endedAt := time.Now().UTC().Format(time.RFC3339)
	index := func(rkey, title string, ended bool, isFirstSync bool) string {
		t.Helper()
		rec := &placestream.Livestream{
			LexiconTypeID: "place.stream.livestream",
			CreatedAt:     createdAt,
			Title:         title,
		}
		if ended {
			rec.EndedAt = &endedAt
		}
		var buf bytes.Buffer
		require.NoError(t, rec.MarshalCBOR(&buf))
		recCBOR := buf.Bytes()
		rcid, err := spid.GetCID(rec)
		require.NoError(t, err)
		require.NoError(t, atsync.handleCreateUpdate(ctx, did, syntax.RecordKey(rkey),
			&recCBOR, rcid.String(), syntax.NSID("place.stream.livestream"), false, isFirstSync, ""))
		return "at://" + did + "/place.stream.livestream/" + rkey
	}
	scheduled := func(uri string) int {
		t.Helper()
		tasks, err := atsync.StatefulDB.ListTasks(ctx, statedb.TaskFilters{Type: statedb.TaskAutoPublishVOD, Limit: 20})
		require.NoError(t, err)
		n := 0
		for _, task := range tasks {
			if bytes.Contains(task.Payload, []byte(uri)) {
				n++
			}
		}
		return n
	}

	optIn(true)
	live := index("3llive0000000", "live", false, false)
	require.Zero(t, scheduled(live), "still live")
	index("3llive0000000", "live", true, false)
	require.Equal(t, 1, scheduled(live), "seen ending")
	index("3llive0000000", "live", true, false)
	index("3llive0000000", "live, retitled", true, false)
	require.Equal(t, 1, scheduled(live), "seen again, and edited")

	first := index("3lfirst000000", "first", true, false)
	require.Equal(t, 1, scheduled(first), "first seen already ended")

	old := index("3lold00000000", "old", true, true)
	require.Zero(t, scheduled(old), "met ended in a backfill")
	index("3lold00000000", "old", true, false)
	require.Zero(t, scheduled(old), "the backfilled record seen again")

	optIn(false)
	off := index("3loff00000000", "off", false, false)
	index("3loff00000000", "off", true, false)
	require.Zero(t, scheduled(off), "ended with the setting off")
	optIn(true)
	index("3loff00000000", "off, retitled", true, false)
	require.Zero(t, scheduled(off), "edited after turning it on")
}
