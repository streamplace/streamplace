package atproto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
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
	vc := &streamplace.LiveViewCount{
		LexiconTypeID: "place.stream.live.viewCount",
		Count:         42,
		Server:        "did:web:server1.example.com",
		Streamer:      "did:plc:abc123",
		UpdatedAt:     &updatedAt,
	}
	err = CommitServerRepoRecord(context.Background(), &cli, "place.stream.live.viewCount", "did:plc:abc123", vc)
	require.NoError(t, err)

	// Read it back
	out, err := ServerRepoGetRecord(context.Background(), "did:web:server1.example.com", "place.stream.live.viewCount", "did:plc:abc123")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Contains(t, out.Uri, "place.stream.live.viewCount")

	// List records
	listOut, err := ServerRepoListRecords(context.Background(), "place.stream.live.viewCount", "", 100, "did:web:server1.example.com", nil)
	require.NoError(t, err)
	require.Len(t, listOut.Records, 1)

	// Merkle proof
	proof, err := ServerRepoMerkleProof(context.Background(), "place.stream.live.viewCount", "did:plc:abc123")
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
	out, err = ServerRepoGetRecord(context.Background(), "did:web:server1.example.com", "place.stream.live.viewCount", "did:plc:abc123")
	require.NoError(t, err)
	require.NotNil(t, out)

	handle.Close()
}
