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

// TestAutoPublishVODScheduledWhenSeenEnding: sync queues the automatic-VOD
// decision when it first sees a livestream record ended, and only then. A
// record met already ended in a backfill is history; seeing it again later
// must not publish a recording from before the streamer opted in.
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
	// Fixed times, so indexing the same record again is byte-identical.
	createdAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	endedAt := time.Now().UTC().Format(time.RFC3339)
	index := func(rkey string, ended bool, isFirstSync bool) string {
		t.Helper()
		rec := &placestream.Livestream{
			LexiconTypeID: "place.stream.livestream",
			CreatedAt:     createdAt,
			Title:         "stream " + rkey,
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
		tasks, err := atsync.StatefulDB.ListTasks(ctx, statedb.TaskFilters{Type: statedb.TaskAutoPublishVOD, Limit: 10})
		require.NoError(t, err)
		n := 0
		for _, task := range tasks {
			if bytes.Contains(task.Payload, []byte(uri)) {
				n++
			}
		}
		return n
	}

	live := index("3llive0000000", false, false)
	require.Zero(t, scheduled(live), "still live")

	old := index("3lold00000000", true, true)
	require.Zero(t, scheduled(old), "met ended in a backfill")
	index("3lold00000000", true, false)
	require.Zero(t, scheduled(old), "the backfilled record seen again")

	ended := index("3lended000000", true, false)
	require.Equal(t, 1, scheduled(ended), "seen ending")
	index("3lended000000", true, false)
	require.Equal(t, 1, scheduled(ended), "seen again")
}
