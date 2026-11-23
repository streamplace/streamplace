package atproto

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

type firstLivestreamPush struct {
	targets []notifications.NotificationTarget
	blast   *notifications.NotificationBlast
}

type firstLivestreamInbox chan firstLivestreamPush

func (inbox firstLivestreamInbox) Blast(ctx context.Context, targets []notifications.NotificationTarget, blast *notifications.NotificationBlast) error {
	select {
	case inbox <- firstLivestreamPush{targets: targets, blast: blast}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestFirstLivestreamPush(t *testing.T) {
	atsync, mod, _ := offlineSynchronizer(t)
	inbox := make(firstLivestreamInbox, 8)
	atsync.Noter = inbox
	admin := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	otherAdmin := "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	streamer := "did:plc:cccccccccccccccccccccccc"
	returning := "did:plc:dddddddddddddddddddddddd"
	denied := "did:plc:eeeeeeeeeeeeeeeeeeeeeeee"
	atsync.CLI.AdminDIDs = []string{admin, otherAdmin}
	atsync.CLI.AllowedStreams = []string{streamer, returning}
	for _, did := range []string{streamer, returning, denied} {
		require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Handle: "streamer.test", Version: "3lrev00000000"}))
	}
	require.NoError(t, atsync.StatefulDB.CreateAuthenticatedNotification("admin-fcm", admin, notifications.NotificationTypeFirebase))
	require.NoError(t, atsync.StatefulDB.CreateAuthenticatedNotification("admin-web", otherAdmin, notifications.NotificationTypeWeb))
	require.NoError(t, atsync.StatefulDB.CreateAuthenticatedNotification("streamer-fcm", streamer, notifications.NotificationTypeFirebase))
	require.NoError(t, atsync.StatefulDB.CreateNotification("unverified-web", admin, notifications.NotificationTypeWeb))

	index := func(did, rkey, title string, update, backfill bool) {
		t.Helper()
		const collection = "place.stream.livestream"
		rec := &placestream.Livestream{LexiconTypeID: collection, CreatedAt: "2026-01-01T00:00:00Z", Title: title}
		var buf bytes.Buffer
		require.NoError(t, rec.MarshalCBOR(&buf))
		data := buf.Bytes()
		cid, err := spid.GetCID(rec)
		require.NoError(t, err)
		require.NoError(t, atsync.handleCreateUpdate(t.Context(), did, syntax.RecordKey(rkey), &data, cid.String(), collection, update, backfill, ""))
	}
	index(streamer, "first", "Debut", false, false)
	select {
	case push := <-inbox:
		require.ElementsMatch(t, []notifications.NotificationTarget{
			{Token: "admin-fcm", Type: notifications.NotificationTypeFirebase},
			{Token: "admin-web", Type: notifications.NotificationTypeWeb},
		}, push.targets)
		require.Contains(t, push.blast.Title, "@streamer.test")
		require.Equal(t, "Debut", push.blast.Body)
		require.Equal(t, "/streamer.test", push.blast.Data["path"])
	case <-time.After(5 * time.Second):
		t.Fatal("station admins did not get a first-livestream push")
	}
	index(streamer, "first", "Debut", false, false)
	index(streamer, "first", "Debut, edited", true, false)
	index(streamer, "second", "Next stream", false, false)
	index(returning, "old", "Found in backfill", false, true)
	index(returning, "next", "Returning streamer", false, false)
	index(denied, "first", "Not allowed here", false, false)
	select {
	case push := <-inbox:
		t.Fatalf("unexpected push: %+v", push.blast)
	case <-time.After(250 * time.Millisecond):
	}
}
