package model

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestExpiredPinDoesNotRestoreOlderPin(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	expired := now.Add(-time.Minute)
	for _, pin := range []PinnedRecord{
		{Uri: "at://did:plc:streamer/place.stream.chat.pinnedRecord/1", RepoDID: "did:plc:streamer", CID: "old", CreatedAt: now.Add(-time.Hour)},
		{Uri: "at://did:plc:streamer/place.stream.chat.pinnedRecord/2", RepoDID: "did:plc:streamer", CID: "new", CreatedAt: now, ExpiresAt: &expired},
	} {
		require.NoError(t, db.CreatePinnedRecord(ctx, &pin))
	}

	pin, err := db.GetActivePinnedRecord(ctx, "did:plc:streamer")
	require.NoError(t, err)
	require.Nil(t, pin, "the newest pin supersedes older pins even after it expires")
}

func TestBroadcastScopedPin(t *testing.T) {
	const streamer = "did:plc:streamer"
	root := "at://did:plc:streamer/place.stream.livestream/1"
	const chapter = "at://did:plc:streamer/place.stream.livestream/2"
	ended := "2026-01-01T01:00:00Z"
	expired := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name      string
		latestURI string
		broadcast *string
		endedAt   *string
		expiresAt *time.Time
		want      bool
	}{
		{name: "first chapter", latestURI: root, want: true},
		{name: "later chapter keeps the pin", latestURI: chapter, broadcast: &root, want: true},
		{name: "broadcast scope takes precedence over expiry", latestURI: chapter, broadcast: &root, expiresAt: &expired, want: true},
		{name: "last chapter ends the broadcast", latestURI: chapter, broadcast: &root, endedAt: &ended},
		{name: "new broadcast does not restore the pin", latestURI: chapter},
		{name: "no livestream"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := indexedTestDB(t)
			ctx := context.Background()
			require.NoError(t, db.UpdateRepo(&Repo{DID: streamer, Handle: "streamer.test"}))
			if tt.latestURI != "" {
				rec := placestream.Livestream{
					LexiconTypeID: "place.stream.livestream",
					Title:         "chapter",
					CreatedAt:     "2026-01-01T00:00:00Z",
					Broadcast:     tt.broadcast,
					EndedAt:       tt.endedAt,
				}
				var buf bytes.Buffer
				require.NoError(t, rec.MarshalCBOR(&buf))
				body := buf.Bytes()
				require.NoError(t, db.CreateLivestream(ctx, &Livestream{
					URI: tt.latestURI, CID: "chapter", RepoDID: streamer,
					CreatedAt: time.Now(), Livestream: &body,
				}))
			}
			require.NoError(t, db.CreatePinnedRecord(ctx, &PinnedRecord{
				Uri: "at://did:plc:streamer/place.stream.chat.pinnedRecord/1", CID: "pin",
				RepoDID: streamer, CreatedAt: time.Now(), LivestreamURI: root, ExpiresAt: tt.expiresAt,
			}))

			pin, err := db.GetActivePinnedRecord(ctx, streamer)
			require.NoError(t, err)
			if !tt.want {
				require.Nil(t, pin)
				return
			}
			require.NotNil(t, pin)
			record, err := pin.ToStreamplacePinnedRecord()
			require.NoError(t, err)
			require.NotNil(t, record.Livestream)
			require.Equal(t, root, *record.Livestream)
			view, err := pin.ToStreamplacePinnedRecordView()
			require.NoError(t, err)
			require.NotNil(t, view.Record.Livestream)
			require.Equal(t, root, *view.Record.Livestream)
		})
	}
}

func TestBroadcastPinDoesNotIgnoreInvalidLivestreamData(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	body := []byte("invalid CBOR")
	root := "at://did:plc:streamer/place.stream.livestream/1"
	require.NoError(t, db.CreateLivestream(ctx, &Livestream{
		URI: root, CID: "broken", RepoDID: "did:plc:streamer", Livestream: &body,
	}))
	require.NoError(t, db.CreatePinnedRecord(ctx, &PinnedRecord{
		Uri: "at://did:plc:streamer/place.stream.chat.pinnedRecord/1", CID: "pin",
		RepoDID: "did:plc:streamer", LivestreamURI: root,
	}))
	pin, err := db.GetActivePinnedRecord(ctx, "did:plc:streamer")
	require.Error(t, err)
	require.Nil(t, pin)
}
