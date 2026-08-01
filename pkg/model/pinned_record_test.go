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
func putPinnedRecord(t *testing.T, m Model, uri, streamerDID string, createdAt time.Time, duration, livestreamURI string, expiresAt *time.Time) {
	t.Helper()
	pin := &PinnedRecord{
		Uri:           uri,
		CID:           "bafypin",
		RepoDID:       streamerDID,
		PinnedMessage: "at://did:plc:user/place.stream.chat.message/msg1",
		PinnedBy:      streamerDID,
		CreatedAt:     createdAt,
		Duration:      duration,
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
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", "", nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "legacy pin with no duration/livestream should be active")
	require.Equal(t, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", pin.Uri)
}

func TestGetActivePinnedRecord_ForeverPinAlwaysActive(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	// Create an ended livestream — the forever pin should still be active.
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339)))
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "forever", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "forever pin should be active even if livestream ended")
}

func TestGetActivePinnedRecord_StreamEndWithActiveLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil) // active stream
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "streamEnd pin with active livestream should be active")
}

func TestGetActivePinnedRecord_StreamEndWithEndedLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "streamEnd pin with ended livestream should be inactive")
}

func TestGetActivePinnedRecord_StreamEndWithMissingLivestream(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "", "at://did:plc:streamer/place.stream.livestream/nonexistent", nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "streamEnd pin with missing livestream should be inactive")
}

func TestGetActivePinnedRecord_NewestSupersedesOlder(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const ls1URI = "at://did:plc:streamer/place.stream.livestream/ls1"

	// ls1 has ended.
	putLivestream(t, m, ls1URI, ptrTo(time.Now().UTC().Format(time.RFC3339)))

	// Pin for ended stream (newest) supersedes the older forever pin.
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-1*time.Minute), "forever", "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), "", ls1URI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "most recent pin's stream ended, so no older pin should be resurrected")
}

// TestGetActivePinnedRecord_PinForeverThenPinStream_StreamEnds covers the
// product scenario: pin a message forever, then pin a message scoped to the
// current stream. When the stream ends, the stream pin goes inactive and the
// forever pin must NOT come back — the most recent pin record is
// authoritative.
func TestGetActivePinnedRecord_PinForeverThenPinStream_StreamEnds(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, ptrTo(time.Now().UTC().Format(time.RFC3339))) // ended

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "forever", "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), "", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "stream pin superseded the forever pin; after stream end there is no pin")
}

func TestGetActivePinnedRecord_NewestActiveWins(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil) // active

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "forever", "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), "", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin, "most recent pin is active, so it wins over the older forever pin")
	require.Equal(t, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", pin.Uri)
}

func TestGetActivePinnedRecord_ExpiredPinSkipped(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	past := time.Now().Add(-1 * time.Hour)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "forever", "", &past)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "expired pin should not be returned even if duration is forever")
}

func TestGetActivePinnedRecord_NewestExpiredSupersedesOlder(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	past := time.Now().Add(-1 * time.Hour)

	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now().Add(-2*time.Minute), "forever", "", nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p2", streamer, time.Now(), "forever", "", &past)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.Nil(t, pin, "expired newest pin supersedes the older active one")
}

func TestGetActivePinnedRecord_DurationAndLivestreamRoundTrip(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const streamer = "did:plc:streamer"
	const lsURI = "at://did:plc:streamer/place.stream.livestream/ls1"
	putLivestream(t, m, lsURI, nil)
	putPinnedRecord(t, m, "at://did:plc:streamer/place.stream.chat.pinnedRecord/p1", streamer, time.Now(), "forever", lsURI, nil)

	pin, err := m.GetActivePinnedRecord(ctx, streamer)
	require.NoError(t, err)
	require.NotNil(t, pin)

	pr, err := pin.ToStreamplacePinnedRecord()
	require.NoError(t, err)
	require.NotNil(t, pr.Duration)
	require.Equal(t, "forever", *pr.Duration)
	require.NotNil(t, pr.Livestream)
	require.Equal(t, lsURI, *pr.Livestream)
}
