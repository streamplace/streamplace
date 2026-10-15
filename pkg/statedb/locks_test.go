package statedb

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

var lockRuns = 100
var nodeCount = 25

func TestPostgresLocks(t *testing.T) {
	if postgresURL == "" {
		t.Skip("no postgres url, skipping postgres tests")
		return
	}
	dburl := makePostgresURL(t)
	cli := config.CLI{
		DBURL: dburl,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var g errgroup.Group
	var count atomic.Uint64
	start := make(chan struct{})
	for i := 0; i < nodeCount; i++ {
		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)
		state, err := MakeDB(ctx, &cli, nil, mod)
		require.NoError(t, err)

		defer func() {
			sqlDB, err := state.DB.DB()
			require.NoError(t, err)
			err = sqlDB.Close()
			require.NoError(t, err)
		}()

		doLock := func() error {
			<-start
			unlock, err := state.GetNamedLock("test")
			require.NoError(t, err)
			defer unlock()
			count.Add(1)
			return nil
		}

		for i := 0; i < lockRuns; i++ {
			g.Go(doLock)
		}
	}
	close(start)

	err := g.Wait()
	require.NoError(t, err)
	require.Equal(t, int(count.Load()), int(uint64(lockRuns*nodeCount)))

}
