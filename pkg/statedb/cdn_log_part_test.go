package statedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

func TestCDNLogPartClaimLifecycle(t *testing.T) {
	cli := &config.CLI{DBURL: ":memory:"}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(t.Context(), cli, nil, mod)
	require.NoError(t, err)
	ctx := context.Background()

	// First claim wins; a concurrent second claim loses.
	ok, err := state.ClaimPart(ctx, "bunny", "p/1.gzip", time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = state.ClaimPart(ctx, "bunny", "p/1.gzip", time.Hour)
	require.NoError(t, err)
	require.False(t, ok)

	// A stale claim (staleAfter elapsed) can be retaken.
	ok, err = state.ClaimPart(ctx, "bunny", "p/1.gzip", 0)
	require.NoError(t, err)
	require.True(t, ok)

	// Completed parts are never reclaimed, even with staleAfter=0.
	require.NoError(t, state.CompletePart(ctx, "bunny", "p/1.gzip", 42))
	ok, err = state.ClaimPart(ctx, "bunny", "p/1.gzip", 0)
	require.NoError(t, err)
	require.False(t, ok)
	var row CDNLogPart
	require.NoError(t, state.DB.Where("source = ? AND part_id = ?", "bunny", "p/1.gzip").First(&row).Error)
	require.NotNil(t, row.CompletedAt)
	require.Equal(t, 42, row.Events)

	// Release drops an in-progress claim so it can be retaken at once;
	// release on a completed part is a no-op.
	ok, err = state.ClaimPart(ctx, "bunny", "p/2.gzip", time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, state.ReleasePart(ctx, "bunny", "p/2.gzip"))
	ok, err = state.ClaimPart(ctx, "bunny", "p/2.gzip", time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, state.ReleasePart(ctx, "bunny", "p/1.gzip"))
	ok, err = state.ClaimPart(ctx, "bunny", "p/1.gzip", 0)
	require.NoError(t, err)
	require.False(t, ok, "completed part survives a release")

	// Sources are independent namespaces.
	ok, err = state.ClaimPart(ctx, "other", "p/1.gzip", time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
}
