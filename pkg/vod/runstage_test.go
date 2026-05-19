package vod

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/log"
)

// TestRunVODStage covers the post-pipeline stage watchdog: it returns the
// stage's own result, and a context cancellation unblocks a stage that
// honors the deadline rather than waiting out vodStageTimeout. (The
// heartbeat is logging-only and not asserted here.)
func TestRunVODStage(t *testing.T) {
	ctx := log.WithLogValues(context.Background(), "test", "TestRunVODStage")

	t.Run("success", func(t *testing.T) {
		require.NoError(t, runVODStage(ctx, "ok", func(ctx context.Context) error {
			return nil
		}))
	})

	t.Run("propagates the stage error", func(t *testing.T) {
		sentinel := errors.New("boom")
		err := runVODStage(ctx, "fail", func(ctx context.Context) error {
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("cancellation unblocks a ctx-aware stage", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		err := runVODStage(cctx, "blocked", func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
		require.ErrorIs(t, err, context.Canceled)
		require.Less(t, time.Since(start), vodStageTimeout,
			"should return when the context is cancelled, not wait out vodStageTimeout")
	})
}
