package atproto

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

type originCommitDuringReindex struct {
	model.Model
	commit func() error
}

func (m *originCommitDuringReindex) UpsertOwnMediaOrigin(ctx context.Context, serverDID, blobCID string, size int64, mimeType string) error {
	if m.commit != nil {
		commit := m.commit
		m.commit = nil
		if err := commit(); err != nil {
			return err
		}
	}
	return m.Model.UpsertOwnMediaOrigin(ctx, serverDID, blobCID, size, mimeType)
}

// Live repair must read a stable snapshot without holding the repo writer lock
// while it writes the index. A commit during the first upsert must succeed, but
// its new origin belongs to the next snapshot rather than changing this walk.
func TestReindexOriginsConcurrentCommit(t *testing.T) {
	ctx := context.Background()
	cli := config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      "origins.example.com",
		DBURL:           ":memory:",
		DataDir:         t.TempDir(),
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, &cli, nil, mod)
	require.NoError(t, err)
	handle, err := MakeServerRepo(ctx, &cli, state)
	require.NoError(t, err)
	defer func() { require.NoError(t, handle.Close()) }()
	t.Cleanup(func() {
		ServerRepo = nil
		ServerCarStore = nil
		ServerPubMultibase = ""
	})

	put := func(blob string) error {
		return CommitServerRepoRecord(ctx, &cli, constants.PLACE_STREAM_MEDIA_ORIGIN, blob, &placestream.MediaOrigin{
			LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
			Blob:          blob,
			Size:          123,
			MimeType:      "video/mp4",
		})
	}
	require.NoError(t, put("blobA"))
	require.NoError(t, put("blobC"))
	wrapped := &originCommitDuringReindex{Model: mod, commit: func() error { return put("blobB") }}
	result, err := ReindexOwnMediaOrigins(ctx, wrapped, cli.ServerDID())
	require.NoError(t, err)
	require.Empty(t, result.Errors)
	require.Equal(t, 2, result.Indexed)
	for _, blob := range []string{"blobA", "blobC"} {
		origin, err := mod.GetMediaOriginByURI(ctx, fmt.Sprintf("at://%s/%s/%s", cli.ServerDID(), constants.PLACE_STREAM_MEDIA_ORIGIN, blob))
		require.NoError(t, err)
		require.Equal(t, blob, origin.Blob)
	}
	newOrigin, err := mod.GetMediaOriginByURI(ctx, fmt.Sprintf("at://%s/%s/blobB", cli.ServerDID(), constants.PLACE_STREAM_MEDIA_ORIGIN))
	require.NoError(t, err)
	require.Empty(t, newOrigin.Blob, "a concurrent commit is not part of the captured snapshot")

	result, err = ReindexOwnMediaOrigins(ctx, mod, cli.ServerDID())
	require.NoError(t, err)
	require.Empty(t, result.Errors)
	require.Equal(t, 3, result.Indexed)
	newOrigin, err = mod.GetMediaOriginByURI(ctx, fmt.Sprintf("at://%s/%s/blobB", cli.ServerDID(), constants.PLACE_STREAM_MEDIA_ORIGIN))
	require.NoError(t, err)
	require.Equal(t, "blobB", newOrigin.Blob)
}
