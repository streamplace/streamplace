package statedb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/indexdb"
)

// TestIndexOwnMediaOrigin checks the seam pkg/vod uses to index an origin
// without waiting on the firehose. The authority must be our ServerDID — that
// is the exact (server_did, blob) key getVideoList filters on, so getting it
// wrong reintroduces the silent invisibility this exists to prevent.
func TestIndexOwnMediaOrigin(t *testing.T) {
	ctx := context.Background()
	cli := config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      "server1.example.com",
		DBURL:           ":memory:",
	}
	cli.DataDir = t.TempDir()

	mod, err := indexdb.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(ctx, &cli, nil, mod)
	require.NoError(t, err)

	require.NoError(t, state.IndexOwnMediaOrigin(ctx, "blobXYZ", 4096, "video/mp4"))

	origin, err := mod.GetMediaOriginByURI(ctx,
		"at://did:web:server1.example.com/"+constants.PLACE_STREAM_MEDIA_ORIGIN+"/blobXYZ")
	require.NoError(t, err)
	require.Equal(t, "blobXYZ", origin.Blob)
	require.Equal(t, int64(4096), origin.Size)
	require.Equal(t, "video/mp4", origin.MimeType)

	// It is the row getVideoList looks for, under our DID and no other.
	hosted, err := mod.GetVideoList(ctx, "", 25, "", "did:web:server1.example.com")
	require.NoError(t, err)
	require.Empty(t, hosted.Videos) // no video records seeded; the point is it doesn't error

	// Idempotent: publishing the same origin twice (retry, or the firehose
	// copy landing afterward) must not duplicate or fail.
	require.NoError(t, state.IndexOwnMediaOrigin(ctx, "blobXYZ", 4096, "video/mp4"))

	origins, err := mod.GetMediaOriginsByBlob(ctx, "blobXYZ")
	require.NoError(t, err)
	require.Len(t, origins, 1)
}

// TestIndexOwnMediaOriginNoModel covers the standalone/microservice case: with
// no index attached there is nothing to write, and that must not be an error.
func TestIndexOwnMediaOriginNoModel(t *testing.T) {
	ctx := context.Background()
	cli := config.CLI{ServerHost: "server1.example.com", DBURL: ":memory:"}
	cli.DataDir = t.TempDir()

	state, err := MakeDB(ctx, &cli, nil, nil)
	require.NoError(t, err)
	require.NoError(t, state.IndexOwnMediaOrigin(ctx, "blobXYZ", 1, "video/mp4"))
}
