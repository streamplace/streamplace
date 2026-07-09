package atproto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/placestream"
)

func TestServerRepo(t *testing.T) {
	cli := config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      "server1.example.com",
		DBURL:           ":memory:",
	}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)

	// Create a new server repo
	handle, err := MakeServerRepo(context.Background(), &cli, state)
	require.NoError(t, err)
	require.NotNil(t, ServerRepo)
	require.NotEmpty(t, ServerPubMultibase)
	require.Equal(t, "did:web:server1.example.com", ServerRepo.RepoDid())

	// Verify the repo can be opened and read
	r, ses, err := OpenServerRepo(context.Background())
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NotNil(t, ses)

	// Get the full repo as CAR
	carBytes, err := ServerRepoGetRepo(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, carBytes)

	// Put a LiveViewCount record
	updatedAt := "2026-03-21T00:00:00Z"
	vc := &streamplace.LiveViewerCount{
		LexiconTypeID: constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
		Count:         42,
		Server:        "did:web:server1.example.com",
		Streamer:      "did:plc:abc123",
		UpdatedAt:     &updatedAt,
	}
	err = CommitServerRepoRecord(context.Background(), &cli, constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "did:plc:abc123", vc)
	require.NoError(t, err)

	// Read it back
	out, err := ServerRepoGetRecord(context.Background(), "did:web:server1.example.com", constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "did:plc:abc123")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Contains(t, out.Uri, constants.PLACE_STREAM_LIVE_VIEWERCOUNT)

	// List records
	listOut, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "", 100, "did:web:server1.example.com", nil)
	require.NoError(t, err)
	require.Len(t, listOut.Records, 1)

	// ListCollections should report the collection we just wrote.
	cols, err := ServerRepoListCollections(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{constants.PLACE_STREAM_LIVE_VIEWERCOUNT}, cols)

	// Merkle proof
	proof, err := ServerRepoMerkleProof(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "did:plc:abc123")
	require.NoError(t, err)
	require.NotEmpty(t, proof)

	// Verify commit events were stored
	evts, err := GetServerCommitEventsSinceSeq("did:web:server1.example.com", 0)
	require.NoError(t, err)
	require.Len(t, evts, 1)

	// Close and reopen to test persistence
	handle.Close()

	// Reset globals
	ServerRepo = nil
	ServerCarStore = nil
	ServerPubMultibase = ""

	handle, err = MakeServerRepo(context.Background(), &cli, state)
	require.NoError(t, err)
	require.NotNil(t, ServerRepo)

	// The record should still be there (file-backed carstore)
	out, err = ServerRepoGetRecord(context.Background(), "did:web:server1.example.com", constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "did:plc:abc123")
	require.NoError(t, err)
	require.NotNil(t, out)

	// After writing a record in a second collection, ListCollections
	// should pick both up in sorted order.
	origin := &streamplace.MediaOrigin{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
		Blob:          "babczxv...",
		Size:          1234,
		MimeType:      "video/mp4",
	}
	err = CommitServerRepoRecord(context.Background(), &cli, constants.PLACE_STREAM_MEDIA_ORIGIN, "babczxv1", origin)
	require.NoError(t, err)
	cols, err = ServerRepoListCollections(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{
		constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
		constants.PLACE_STREAM_MEDIA_ORIGIN,
	}, cols)

	// ListRecords must filter strictly by collection. Asking for
	// viewerCount should not surface the media.origin record we just
	// committed.
	vcOnly, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "", 100, "did:web:server1.example.com", nil)
	require.NoError(t, err)
	require.Len(t, vcOnly.Records, 1)
	require.Contains(t, vcOnly.Records[0].Uri, constants.PLACE_STREAM_LIVE_VIEWERCOUNT)

	originOnly, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_MEDIA_ORIGIN, "", 100, "did:web:server1.example.com", nil)
	require.NoError(t, err)
	require.Len(t, originOnly.Records, 1)
	require.Contains(t, originOnly.Records[0].Uri, constants.PLACE_STREAM_MEDIA_ORIGIN)

	handle.Close()
}

// TestServerRepoListRecords_Pagination covers limit + cursor + reverse
// independently of the broader TestServerRepo flow so the contract is
// easy to read in isolation.
func TestServerRepoListRecords_Pagination(t *testing.T) {
	cli := config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      "server2.example.com",
		DBURL:           ":memory:",
	}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)

	// Reset globals from any prior test in this file.
	ServerRepo = nil
	ServerCarStore = nil
	ServerPubMultibase = ""
	handle, err := MakeServerRepo(context.Background(), &cli, state)
	require.NoError(t, err)
	defer handle.Close()

	// Seed five viewerCount records with monotonically increasing rkeys
	// so we can predict the natural ascending order.
	streamers := []string{"did:plc:a", "did:plc:b", "did:plc:c", "did:plc:d", "did:plc:e"}
	updatedAt := "2026-03-21T00:00:00Z"
	for _, s := range streamers {
		vc := &streamplace.LiveViewerCount{
			LexiconTypeID: constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
			Count:         1,
			Server:        "did:web:server2.example.com",
			Streamer:      s,
			UpdatedAt:     &updatedAt,
		}
		require.NoError(t, CommitServerRepoRecord(context.Background(), &cli, constants.PLACE_STREAM_LIVE_VIEWERCOUNT, s, vc))
	}

	// Default order is reverse-lexical (newest TID first). limit=2:
	// first page returns the two largest rkeys + a cursor pointing at
	// the smaller of the two.
	page1, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "", 2, "did:web:server2.example.com", nil)
	require.NoError(t, err)
	require.Len(t, page1.Records, 2)
	require.Contains(t, page1.Records[0].Uri, "did:plc:e")
	require.Contains(t, page1.Records[1].Uri, "did:plc:d")
	require.NotNil(t, page1.Cursor)
	require.Equal(t, "did:plc:d", *page1.Cursor)

	// Pass the cursor back for the second page — keeps going down.
	page2, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, *page1.Cursor, 2, "did:web:server2.example.com", nil)
	require.NoError(t, err)
	require.Len(t, page2.Records, 2)
	require.Contains(t, page2.Records[0].Uri, "did:plc:c")
	require.Contains(t, page2.Records[1].Uri, "did:plc:b")
	require.NotNil(t, page2.Cursor)

	// Third page has the final record; cursor must be omitted since
	// we returned fewer than limit.
	page3, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, *page2.Cursor, 2, "did:web:server2.example.com", nil)
	require.NoError(t, err)
	require.Len(t, page3.Records, 1)
	require.Contains(t, page3.Records[0].Uri, "did:plc:a")
	require.Nil(t, page3.Cursor)

	// reverse=true flips back to lex-ascending (oldest first).
	rev := true
	revOut, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "", 100, "did:web:server2.example.com", &rev)
	require.NoError(t, err)
	require.Len(t, revOut.Records, 5)
	require.Contains(t, revOut.Records[0].Uri, "did:plc:a")
	require.Contains(t, revOut.Records[4].Uri, "did:plc:e")

	// reverse=true also paginates in lex-ascending order.
	revPage1, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "", 2, "did:web:server2.example.com", &rev)
	require.NoError(t, err)
	require.Len(t, revPage1.Records, 2)
	require.Contains(t, revPage1.Records[0].Uri, "did:plc:a")
	require.Contains(t, revPage1.Records[1].Uri, "did:plc:b")
	require.NotNil(t, revPage1.Cursor)
	require.Equal(t, "did:plc:b", *revPage1.Cursor)

	revPage2, err := ServerRepoListRecords(context.Background(), constants.PLACE_STREAM_LIVE_VIEWERCOUNT, *revPage1.Cursor, 2, "did:web:server2.example.com", &rev)
	require.NoError(t, err)
	require.Len(t, revPage2.Records, 2)
	require.Contains(t, revPage2.Records[0].Uri, "did:plc:c")
	require.Contains(t, revPage2.Records[1].Uri, "did:plc:d")
}
