package atproto

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/reposync"
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
		var buf bytes.Buffer
		require.NoError(t, (&placestream.ServerSettings{AutoPublishVods: &on}).MarshalCBOR(&buf))
		rec := buf.Bytes()
		require.NoError(t, mod.UpdateServerSettings(ctx, &model.ServerSettings{Server: atsync.CLI.BroadcasterHost, RepoDID: did, Record: &rec}))
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

func TestDeletedServerSettingsWithdrawAutoPublishConsent(t *testing.T) {
	ctx := context.Background()
	dev := devenv.WithDevEnv(t)
	atsync, mod := backfillTestSynchronizer(t, dev)
	user := dev.CreateAccount(t)
	other := dev.CreateAccount(t)
	on := true
	for _, account := range []*devenv.DevEnvAccount{user, other} {
		for _, host := range []string{atsync.CLI.BroadcasterHost, "other.example.com"} {
			createBackfillRecord(t, account, "place.stream.server.settings", host,
				&placestream.ServerSettings{LexiconTypeID: "place.stream.server.settings", AutoPublishVods: &on})
		}
		_, err := atsync.SyncBlueskyRepoCached(ctx, account.DID)
		require.NoError(t, err)
	}
	settings, err := mod.GetServerSettings(ctx, atsync.CLI.BroadcasterHost, user.DID)
	require.NoError(t, err)
	require.NotNil(t, settings)

	_, err = comatproto.RepoDeleteRecord(ctx, user.XRPC, &comatproto.RepoDeleteRecord_Input{
		Repo: user.DID, Collection: "place.stream.server.settings", Rkey: atsync.CLI.BroadcasterHost,
	})
	require.NoError(t, err)
	blocks, err := comatproto.SyncGetRepo(ctx, user.XRPC, user.DID, "")
	require.NoError(t, err)
	before, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	evt := &indigoatproto.SyncSubscribeRepos_Commit{
		Repo: user.DID, Time: time.Now().UTC().Format(time.RFC3339), Blocks: blocks,
		Since: &before.Version, Rev: reposync.TIDForTime(time.Now().Add(time.Hour)),
		Ops: []*indigoatproto.SyncSubscribeRepos_RepoOp{
			repoOp("delete", "place.stream.server.settings/"+atsync.CLI.BroadcasterHost),
		},
	}
	db := mod.(*model.DBModel).DB
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("fail_settings_delete", func(tx *gorm.DB) {
		tx.Error = errors.New("settings deletion unavailable")
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Delete().Remove("fail_settings_delete"))
	})
	atsync.handleCommitEventOps(ctx, evt)
	after, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, before.Version, after.Version, "a failed consent deletion must not advance the commit watermark")
	require.NoError(t, db.Callback().Delete().Remove("fail_settings_delete"))
	atsync.handleCommitEventOps(ctx, evt)
	after, err = mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, evt.Rev, after.Version, "successful reprocessing can advance the watermark")
	settings, err = mod.GetServerSettings(ctx, atsync.CLI.BroadcasterHost, user.DID)
	require.NoError(t, err)
	require.Nil(t, settings, "deleting the record must withdraw indexed consent")
	require.NoError(t, atsync.StatefulDB.ScheduleAutoPublishVOD(ctx, user.DID, "at://"+user.DID+"/place.stream.livestream/3lended00000"))
	tasks, err := atsync.StatefulDB.ListTasks(ctx, statedb.TaskFilters{Type: statedb.TaskAutoPublishVOD})
	require.NoError(t, err)
	require.Empty(t, tasks, "a deleted opt-in must not schedule publication")
	for _, key := range [][2]string{{"other.example.com", user.DID}, {atsync.CLI.BroadcasterHost, other.DID}} {
		settings, err := mod.GetServerSettings(ctx, key[0], key[1])
		require.NoError(t, err)
		require.NotNil(t, settings, "deletion must be scoped to the server and account")
	}
}
