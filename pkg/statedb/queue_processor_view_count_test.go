package statedb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/config"
)

func TestStationGeneralWorkerViewCountEligibility(t *testing.T) {
	ctx := t.Context()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "queue.db")))
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sql.Close() })
	require.NoError(t, db.AutoMigrate(&AppTask{}, &AppTaskEffect{}))

	cli := &config.CLI{StationDID: "did:web:station.example"}
	state := &StatefulDB{DB: db, Type: DBTypeSQLite, CLI: cli}
	aggregate, err := state.EnqueueTask(ctx, TaskViewCountAggregate, ViewCountAggregateTask{})
	require.NoError(t, err)
	other, err := state.EnqueueTask(ctx, TaskCDNLogIngest, CDNLogIngestTask{})
	require.NoError(t, err)

	// An installed handler cannot make a station non-publisher eligible.
	handler := func(context.Context, ViewCountAggregateTask) error { return nil }
	state.SetViewCountAggregator(handler)
	next, err := state.DequeueTask(ctx, "general", state.generalWorkerTaskTypes()...)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, other.ID, next.ID)
	require.Zero(t, requireTaskStatus(t, state, aggregate.ID, TaskStatusPending).TryCount)

	// A publisher with aggregation disabled must also leave the task alone.
	cli.StationOriginPublisher = true
	state.SetViewCountAggregator(nil)
	next, err = state.DequeueTask(ctx, "general", state.generalWorkerTaskTypes()...)
	require.NoError(t, err)
	require.Nil(t, next)
	require.Zero(t, requireTaskStatus(t, state, aggregate.ID, TaskStatusPending).TryCount)

	standalone := &StatefulDB{CLI: &config.CLI{}}
	require.Contains(t, standalone.generalWorkerTaskTypes(), TaskViewCountAggregate)
	require.Contains(t, standalone.generalWorkerTaskTypes(), TaskCDNLogIngest)

	state.SetViewCountAggregator(handler)
	next, err = state.DequeueTask(ctx, "general", state.generalWorkerTaskTypes()...)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, aggregate.ID, next.ID)
	require.Equal(t, 1, next.TryCount)
}
