package model

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

// putLivestream creates a Livestream row whose CBOR blob is the given record.
// endedAt may be nil for an active stream.
func putLivestream(t *testing.T, m Model, uri string, endedAt *string) {
	t.Helper()
	ctx := context.Background()
	rec := placestream.Livestream{
		LexiconTypeID: "place.stream.livestream",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		EndedAt:       endedAt,
		Title:         "test stream",
	}
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	ls := &Livestream{
		URI:        uri,
		CID:        "bafytest",
		CreatedAt:  time.Now().UTC(),
		Livestream: ptrTo(buf.Bytes()),
		RepoDID:    "did:plc:streamer",
	}
	require.NoError(t, m.CreateLivestream(ctx, ls))
}

func ptrTo[T any](v T) *T { return &v }

// putPinnedRecord inserts a PinnedRecord directly for testing.
func putPinnedRecord(t *testing.T, m Model, uri, streamerDID string, createdAt time.Time, livestreamURI string, expiresAt *time.Time) {
	t.Helper()
	pin := &PinnedRecord{
		Uri:           uri,
		CID:           "bafypin",
		RepoDID:       streamerDID,
		PinnedMessage: "at://did:plc:user/place.stream.chat.message/msg1",
		PinnedBy:      streamerDID,
		CreatedAt:     createdAt,
		LivestreamURI: livestreamURI,
		ExpiresAt:     expiresAt,
	}
	require.NoError(t, m.CreatePinnedRecord(context.Background(), pin))
}

func TestGetActivePinnedRecord_LegacyPinAlwaysActive(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "pin with neither livestream nor expiresAt should be active (permanent)")
	require.Equal(t, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", pin.Uri)
}

// TestGetActivePinnedRecord_PermanentPinIgnoresStreamEnd covers the product
// scenario: a pin with neither livestream nor expiresAt is permanent, so an
// ended livestream belonging to the same streamer must not deactivate it.
func TestGetActivePinnedRecord_PermanentPinIgnoresStreamEnd(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	// Create an ended livestream — the permanent pin should still be active.
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339)))
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "permanent pin should be active even if a livestream ended")
}

func TestGetActivePinnedRecord_StreamEndWithActiveLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil) // active stream
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "livestream-scoped pin with active livestream should be active")
}

func TestGetActivePinnedRecord_StreamEndWithEndedLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "livestream-scoped pin with ended livestream should be inactive")
}

func TestGetActivePinnedRecord_StreamEndWithMissingLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "at://did:plc:streamer/place.stream.livestream/nonexistent", nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "livestream-scoped pin with missing livestream should be inactive")
}

func TestGetActivePinnedRecord_NewestSupersedesOlder(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const ls1URI = "at://did:plc:streamer/place.stream.livestream/ls1"

	// ls1 has ended.
	putLivestream(t, m, ls1URI, ptrTo(time.Now().UTC().Format(time.RFC3339)))

	// Pin for ended stream (newest) supersedes the older permanent pin.
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-1*time.Minute), "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), ls1URI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "most recent pin's stream ended, so no older pin should be resurrected")
}

// TestGetActivePinnedRecord_PermanentThenStreamPin_StreamEnds covers the
// product scenario: pin a message permanently, then pin a message scoped to
// the current stream. When the stream ends, the stream pin goes inactive and
// the permanent pin must NOT come back — the most recent pin record is
// authoritative.
func TestGetActivePinnedRecord_PermanentThenStreamPin_StreamEnds(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "stream pin superseded the permanent pin; after stream end there is no pin")
}

func TestGetActivePinnedRecord_NewestActiveWins(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil) // active

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "most recent pin is active, so it wins over the older permanent pin")
	require.Equal(t, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", pin.Uri)
}

func TestGetActivePinnedRecord_ExpiredTimedPinSkipped(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	past := time.Now().Add(-1 * time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", &past)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "expired timed pin should not be returned")
}

func TestGetActivePinnedRecord_NewestExpiredSupersedesOlder(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	past := time.Now().Add(-1 * time.Hour)

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), "", &past)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "expired newest pin supersedes the older active one")
}

func TestGetActivePinnedRecord_ExpiresAtAndLivestreamRoundTrip(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil)
	future := time.Now().Add(time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), lsURI, &future)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin)

	pr, err := pin.ToStreamplacePinnedRecord()
	require.NoError(t, err)
	require.NotNil(t, pr.ExpiresAt)
	require.NotNil(t, pr.Livestream)
	require.Equal(t, lsURI, *pr.Livestream)
}

func TestGetActivePinnedRecord_TimedPinActiveBeforeExpiry(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	future := time.Now().Add(time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", &future)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "timed pin before expiry should be active")
}

// TestGetActivePinnedRecord_TimedPinSurvivesStreamEnd locks in the behavior
// change from the duration enum: a timed pin has no livestream scope, so the
// streamer's stream ending must not expire it.
func TestGetActivePinnedRecord_TimedPinSurvivesStreamEnd(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended
	future := time.Now().Add(time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", &future)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "timed pin is not scoped to a livestream, so stream end must not expire it")
}

// TestGetActivePinnedRecord_LivestreamBeatsExpiresAt covers the precedence
// chain: a livestream-scoped pin is governed by the livestream even when
// expiresAt has passed.
func TestGetActivePinnedRecord_LivestreamBeatsExpiresAt(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil) // active
	past := time.Now().Add(-1 * time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), lsURI, &past)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "livestream takes precedence over expiresAt; an active stream keeps the pin")
}

// TestGetActivePinnedRecord_LivestreamEndedBeatsExpiresAt is the other half
// of the precedence chain: an ended livestream deactivates the pin even when
// expiresAt is in the future.
func TestGetActivePinnedRecord_LivestreamEndedBeatsExpiresAt(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended
	future := time.Now().Add(time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), lsURI, &future)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "livestream takes precedence over expiresAt; an ended stream deactivates the pin")
}

// TestGetActivePinnedRecord_LegacyDurationColumnIgnored simulates rows written
// before the duration field was removed from the lexicon: they carry a
// duration column value that must not affect the livestream/expiresAt logic.
func TestGetActivePinnedRecord_LegacyDurationColumnIgnored(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const pinURI = "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1"
	// Fresh DBs no longer create the column; add it to simulate a pre-change DB.
	db := m.(*DBModel).DB
	require.NoError(t, db.Exec("ALTER TABLE pinned_records ADD COLUMN duration TEXT").Error)
	putPinnedRecord(t, m, pinURI, streamer, time.Now(), "", nil)
	require.NoError(t, db.Exec("UPDATE pinned_records SET duration = 'forever' WHERE uri = ?", pinURI).Error)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "legacy duration value is ignored; a pin with no livestream/expiresAt is permanent")
}
