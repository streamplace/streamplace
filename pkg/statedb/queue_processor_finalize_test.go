package statedb

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/placestream"
)

// marshalLivestream encodes a placestream.Livestream record to the CBOR blob
// shape the model stores (the same bytes atproto sync decodes via
// lexutil.CborDecodeValue).
func marshalLivestream(t *testing.T, rec *placestream.Livestream) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	return buf.Bytes()
}

// seedLivestream inserts a place.stream.livestream row for did with the given
// lastSeenAt/endedAt/idleTimeoutSeconds, returning its URI. createdAgo sets the
// row's created_at (the column GetLatestLivestreamForRepo orders by) so callers
// can control which record is "latest".
func seedLivestream(t *testing.T, mod indexdb.Model, did, rkey string, createdAgo time.Duration, rec *placestream.Livestream) string {
	t.Helper()
	// ToLivestreamView dereferences ls.Repo.Handle, so the streamer needs a
	// repo row. UpdateRepo upserts on PK (did), creating it if absent.
	require.NoError(t, mod.UpdateRepo(&indexdb.Repo{DID: did, Handle: "handle-" + rkey}))
	uri := "at://" + did + "/place.stream.livestream/" + rkey
	created := time.Now().Add(-createdAgo)
	blob := marshalLivestream(t, rec)
	require.NoError(t, mod.CreateLivestream(context.Background(), &indexdb.Livestream{
		URI:        uri,
		CID:        "bafy-" + rkey,
		CreatedAt:  created,
		Livestream: &blob,
		RepoDID:    did,
	}))
	return uri
}

func ptr[T any](v T) *T { return &v }

// seedSession inserts a non-revoked OAuth session row for did, so the repo
// counts as "has a local session on this node." The finalize guard reschedules
// only when a local session exists (the heartbeat-lag scenario is real only
// for repos this node is actively ingesting). Without this, the stale-but-latest
// guard drops the task instead (firehose-observed, never connected here).
func seedSession(t *testing.T, state *StatefulDB, did string) {
	t.Helper()
	require.NoError(t, state.CreateOAuthSession("jkt-"+did, &oatproxy.OAuthSession{
		DID:               did,
		DownstreamDPoPJKT: "jkt-" + did,
	}))
}

// TestFinalizeLivestreamReschedulesWhenLatestButStale proves the guard added to
// processFinalizeLivestreamTask: when a livestream's lastSeenAt is older than
// its idleTimeoutSeconds but the record is still the streamer's latest, AND
// the repo has a local OAuth session (so heartbeat lag is possible), the
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
		uri := seedLivestream(t, state.model, did, "latest", 1*time.Hour, &placestream.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})

		// A local session exists, so the heartbeat-lag guard applies and
		// the task reschedules instead of ending or dropping.
		seedSession(t, state, did)

		task := &AppTask{ID: 1, Type: TaskFinalizeLivestream, Payload: mustMarshal(t, FinalizeLivestreamTask{
			LivestreamURI: uri,
		})}

		// Must not reach the PDS client (no real PDS is configured, only
		// the session row), and must not error — it reschedules and
		// returns nil.
		err := state.processFinalizeLivestreamTask(ctx, task)
		require.NoError(t, err, "stale-but-latest with a session must reschedule, not error or end")

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
		rec, ok := view.Record.Val.(*placestream.Livestream)
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
		_ = seedLivestream(t, state.model, did, "old", 2*time.Hour, &placestream.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})
		// Newer record: makes "old" no longer latest.
		_ = seedLivestream(t, state.model, did, "new", 1*time.Minute, &placestream.Livestream{
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

// TestFinalizeLivestreamDropsStaleLatestWithoutSession proves the fix for the
// dev-environment log flood: when a stale-but-latest livestream has NO local
// OAuth session (it arrived via firehose sync from an account that never
// connected to this node), the task must be dropped — not rescheduled.
//
// Without a local session there is no heartbeat to wait for (doUpdateLivestream
// only runs inside a local StreamSession) and no write access to end the record.
// Rescheduling would respawn the task every idle window forever — the
// rescheduled key embeds a fresh timestamp, so dedup never fires — flooding the
// logs and growing the task table without bound.
func TestFinalizeLivestreamDropsStaleLatestWithoutSession(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:firehose-only"

		// The record is "latest" (only one for this repo) and its lastSeenAt
		// is well past the 300s idle timeout.
		uri := seedLivestream(t, state.model, did, "latest", 1*time.Hour, &placestream.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})
		// Deliberately NO seedSession: this repo was only seen on the firehose.

		// Create the task row so CompleteTask has something to mark done.
		createdTask, err := state.EnqueueTask(ctx, TaskFinalizeLivestream, FinalizeLivestreamTask{
			LivestreamURI: uri,
		}, WithTaskKey("finalize-livestream::"+uri+"::initial"))
		require.NoError(t, err)

		// Must not error, must not reschedule, must not reach the PDS client.
		// The task is dropped (completed) because there's no session and thus
		// no heartbeat-lag scenario to guard.
		err = state.processFinalizeLivestreamTask(ctx, createdTask)
		require.NoError(t, err, "stale-but-latest with no session must drop, not error or reschedule")

		// No rescheduled (pending) task should have been enqueued. The
		// original task row is COMPLETED; filter to PENDING to assert no
		// reschedule was spawned.
		tasks, err := state.ListTasks(ctx, TaskFilters{Type: TaskFinalizeLivestream, Status: TaskStatusPending, Limit: 10})
		require.NoError(t, err)
		require.Empty(t, tasks, "no reschedule task should be enqueued for a session-less stale-but-latest record")

		// The record must NOT have been ended (no session = no write access).
		ls, err := state.model.GetLivestream(uri)
		require.NoError(t, err)
		view, err := ls.ToLivestreamView()
		require.NoError(t, err)
		rec, ok := view.Record.Val.(*placestream.Livestream)
		require.True(t, ok)
		require.Nil(t, rec.EndedAt, "endedAt must not be set without a local session to write it")
	})
}

// TestFinalizeLivestreamSupersededWithoutSessionDropsNotErrors is a regression
// guard: a superseded record with no session should still be droppable. In
// practice superseded records proceed past the latest-record guard to the
// session-lookup path, where GetSessionByDID returns gorm.ErrRecordNotFound.
// This test documents that the superseded path still errors at session lookup
// (it doesn't silently drop), confirming the no-session drop only applies to
// the stale-but-latest branch.
func TestFinalizeLivestreamSupersededWithoutSessionErrorsAtSessionLookup(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:superseded-nosession"

		// Older record: stale, will be superseded.
		_ = seedLivestream(t, state.model, did, "old", 2*time.Hour, &placestream.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Add(-10 * time.Minute).Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})
		// Newer record: makes "old" no longer latest.
		_ = seedLivestream(t, state.model, did, "new", 1*time.Minute, &placestream.Livestream{
			LexiconTypeID:      "place.stream.livestream",
			CreatedAt:          time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			LastSeenAt:         ptr(time.Now().Format(time.RFC3339)),
			IdleTimeoutSeconds: ptr(int64(300)),
		})
		// No session for this repo.

		oldURI := "at://" + did + "/place.stream.livestream/old"
		task := &AppTask{ID: 2, Type: TaskFinalizeLivestream, Payload: mustMarshal(t, FinalizeLivestreamTask{
			LivestreamURI: oldURI,
		})}

		err := state.processFinalizeLivestreamTask(ctx, task)
		// Superseded records proceed past the guard to session lookup, which
		// fails with gorm.ErrRecordNotFound — the no-session drop only
		// applies to the stale-but-latest branch.
		require.Error(t, err, "superseded record without session should error at session lookup, not drop")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

// smoke: the memory-mode StatefulDB used above needs config.DBURL only for the
// non-draft test bootstrap path; reference it so an unused import can't bite
// if these tests grow.
var _ = config.CLI{}
