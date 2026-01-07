package atproto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

func TestLexiconRepoConcurrentAccess(t *testing.T) {
	cli := config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		DataDir:         t.TempDir(),
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)
	handle, err := MakeLexiconRepo(context.Background(), &cli, mod, state)
	require.NoError(t, err)
	handle.Close()
	ctx := context.Background()
	collection := "com.atproto.lexicon.schema"

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < 10; i++ {
		g.Go(func() error {
			res, err := LexiconRepoMerkleProof(ctx, collection, "place.stream.chat.message")
			require.NoError(t, err)
			require.NotNil(t, res)
			return nil
		})
		g.Go(func() error {
			res, err := LexiconRepoListRecords(ctx, collection, "", 10, "did:web:example.com", nil)
			require.NoError(t, err)
			require.NotNil(t, res)
			return nil
		})
		g.Go(func() error {
			res, err := LexiconRepoGetRecord(ctx, "did:web:example.com", collection, "place.stream.chat.message")
			require.NoError(t, err)
			require.NotNil(t, res)
			return nil
		})
	}
	err = g.Wait()
	require.NoError(t, err)
}
