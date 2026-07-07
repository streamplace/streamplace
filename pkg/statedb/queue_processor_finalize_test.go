package statedb

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

// marshalLivestream encodes a streamplace.Livestream record to the CBOR blob
// shape the model stores (the same bytes atproto sync decodes via
// lexutil.CborDecodeValue).
func marshalLivestream(t *testing.T, rec *streamplace.Livestream) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	return buf.Bytes()
}

// seedLivestream inserts a place.stream.livestream row for did with the given
// lastSeenAt/endedAt/idleTimeoutSeconds, returning its URI. createdAgo sets the
// row's created_at (the column GetLatestLivestreamForRepo orders by) so callers
// can control which record is "latest".
func seedLivestream(t *testing.T, mod model.Model, did, rkey string, createdAgo time.Duration, rec *streamplace.Livestream) string {
	t.Helper()
	// ToLivestreamView dereferences ls.Repo.Handle, so the streamer needs a
	// repo row. UpdateRepo upserts on PK (did), creating it if absent.
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Handle: "handle-" + rkey}))
	uri := "at://" + did + "/place.stream.livestream/" + rkey
	created := time.Now().Add(-createdAgo)
	blob := marshalLivestream(t, rec)
	require.NoError(t, mod.CreateLivestream(context.Background(), &model.Livestream{
		URI:        uri,
		CID:        "bafy-" + rkey,
		CreatedAt:  created,
		Livestream: &blob,
		RepoDID:    did,
	}))
	return uri
}

func ptr[T any](v T) *T { return &v }

// TestFinalizeLivestreamReschedulesWhenLatestButStale proves the guard added to
// processFinalizeLivestreamTask: when a livestream's lastSeenAt is older than
// its idleTimeoutSeconds but the record is still the streamer's latest, the
// task must NOT set endedAt (which would take the active stream pre-live). It
// must instead reschedule itself for one more idle window and return nil, so a
// heartbeat that's lagging behind actual ingestion gets a chance to catch up.
//
// This is the exact failure that ended Eli's active stream on 2026-07-06: a
// ~60s ingest gap froze lastSeenAt, the idle timer fired while the stream was
// still flowing, and endedAt was written onto the live record.
func TestFinalizeLivestreamReschedulesWhenLatestButStale(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:reschedule"

		// The record is "latest" (only one for this repo) and its lastSeenAt
		// is well past the 300s idle timeout.
		uri := seedLivestream(t, state.model, did, "latest", 1*time.Hour, &streamplace.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})

		task := &AppTask{ID: 1, Type: TaskFinalizeLivestream, Payload: mustMarshal(t, FinalizeLivestreamTask{
			LivestreamURI: uri,
		})}

		// Must not reach the PDS client (no session is configured), and must
		// not error — it reschedules and returns nil.
		err := state.processFinalizeLivestreamTask(ctx, task)
		require.NoError(t, err, "stale-but-latest must reschedule, not error or end")

		// A rescheduled task must have been enqueued, keyed to the same URI.
		tasks, err := state.ListTasks(ctx, TaskFilters{Type: TaskFinalizeLivestream, Limit: 10})
		require.NoError(t, err)
		require.Len(t, tasks, 1, "exactly one rescheduled finalize task expected")
		require.Contains(t, *tasks[0].TaskKey, "finalize-livestream::"+uri+"::")
		require.NotNil(t, tasks[0].ScheduledAt, "rescheduled task must carry a future ScheduledAt")
		require.True(t, tasks[0].ScheduledAt.After(time.Now()), "rescheduled task must run in the future")

		// And critically: the record must NOT have been ended. Re-read it from
		// the repo and confirm endedAt is still unset.
		ls, err := state.model.GetLivestream(uri)
		require.NoError(t, err)
		view, err := ls.ToLivestreamView()
		require.NoError(t, err)
		rec, ok := view.Record.Val.(*streamplace.Livestream)
		require.True(t, ok)
		require.Nil(t, rec.EndedAt, "endedAt must not be set on a stale-but-latest record")
	})
}

// TestFinalizeLivestreamEndsSupersededRecord proves the complementary case: when
// a newer livestream exists for the repo, the stale older record is no longer
// "latest" and is safe to end — so the guard must NOT reschedule. Instead the
// task proceeds toward ending it (which here surfaces as a session-lookup error,
// since no PDS session is wired; what matters is that no reschedule was enqueued
// and the record wasn't protected by the latest-record guard).
func TestFinalizeLivestreamEndsSupersededRecord(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:superseded"

		// Older record: stale lastSeenAt, but a newer record will exist.
		_ = seedLivestream(t, state.model, did, "old", 2*time.Hour, &streamplace.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})
		// Newer record: makes "old" no longer latest.
		_ = seedLivestream(t, state.model, did, "new", 1*time.Minute, &streamplace.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})

		oldURI := "at://" + did + "/place.stream.livestream/old"
		task := &AppTask{ID: 2, Type: TaskFinalizeLivestream, Payload: mustMarshal(t, FinalizeLivestreamTask{
			LivestreamURI: oldURI,
		})}

		// No reschedule should be enqueued for a superseded record. The task
		// proceeds past the guard and fails at session lookup (no PDS wired),
		// which is the expected "didn't take the guard path" signal.
		err := state.processFinalizeLivestreamTask(ctx, task)
		require.Error(t, err, "superseded record should proceed to end (here: fail at session lookup), not reschedule")

		tasks, err := state.ListTasks(ctx, TaskFilters{Type: TaskFinalizeLivestream, Limit: 10})
		require.NoError(t, err)
		require.Empty(t, tasks, "no reschedule task should be enqueued for a superseded record")
	})
}

// smoke: the memory-mode StatefulDB used above needs config.DBURL only for the
// non-draft test bootstrap path; reference it so an unused import can't bite
// if these tests grow.
var _ = config.CLI{}
