package stt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeModel struct{ info ModelInfo }

func (m *fakeModel) Info() ModelInfo { return m.info }
func (m *fakeModel) Transcribe(context.Context, []float32, Options) (*Result, error) {
	panic("scheduler must not transcribe")
}
func fakeScheduler(budget float64) *scheduler {
	ready := make(chan struct{})
	close(ready)
	done := make(chan struct{})
	close(done)
	e := &scheduler{budget: budget, batchCost: 2, ready: ready, done: done, cancel: func() {}, changed: make(chan struct{}), leases: make(map[*engineLease]struct{})}
	for i, name := range []string{"tiny", "base", "small"} {
		cost := []float64{.5, 1, 2}[i]
		e.models = append(e.models, scheduledModel{model: &fakeModel{info: ModelInfo{Name: name, RealtimeFactor: cost / 4}}, cost: cost})
	}
	return e
}
func TestSchedulerBudgetAndHysteresis(t *testing.T) {
	e := fakeScheduler(4)
	defer e.Close()
	ctx := context.Background()
	first, err := e.Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	second, err := e.Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	require.Equal(t, "small", first.Model().Info().Name)
	third, err := e.Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	require.Equal(t, "base", first.Model().Info().Name)
	require.Equal(t, "base", second.Model().Info().Name)
	require.Equal(t, "base", third.Model().Info().Name)
	third.Release()
	require.Equal(t, "base", first.Model().Info().Name, "20%% spare capacity required before upgrading")
	second.Release()
	require.Equal(t, "small", first.Model().Info().Name)
	first.Release()
	first.Release()
	require.Nil(t, first.Model())
}
func TestSchedulerRefusesUnfitAndPinnedLeases(t *testing.T) {
	e := fakeScheduler(1)
	defer e.Close()
	_, err := e.Lease(context.Background(), LeaseOptions{Realtime: true, Model: "small"})
	require.ErrorIs(t, err, ErrOverBudget)
	first, err := e.Lease(context.Background(), LeaseOptions{Realtime: true, Model: "base"})
	require.NoError(t, err)
	_, err = e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.ErrorIs(t, err, ErrOverBudget)
	require.Equal(t, "base", first.Model().Info().Name)
	first.Release()
	a, err := e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.NoError(t, err)
	b, err := e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.NoError(t, err)
	require.Equal(t, "tiny", a.Model().Info().Name)
	require.Equal(t, "tiny", b.Model().Info().Name)
	_, err = e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.ErrorIs(t, err, ErrOverBudget)
	_, err = e.Lease(context.Background(), LeaseOptions{Model: "missing"})
	require.ErrorContains(t, err, "unknown speech model")
}
func TestBatchLeaseWaitsAndUsesBestModel(t *testing.T) {
	e := fakeScheduler(2)
	defer e.Close()
	live, err := e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = e.Lease(ctx, LeaseOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	live.Release()
	batch, err := e.Lease(context.Background(), LeaseOptions{})
	require.NoError(t, err)
	require.Equal(t, "small", batch.Model().Info().Name)
	_, err = e.Lease(context.Background(), LeaseOptions{Realtime: true})
	require.ErrorIs(t, err, ErrOverBudget)
	batch.Release()
}
func TestCloseWakesWaitingLease(t *testing.T) {
	e := fakeScheduler(2)
	batch, err := e.Lease(context.Background(), LeaseOptions{})
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() { _, err := e.Lease(context.Background(), LeaseOptions{}); result <- err }()
	require.NoError(t, e.Close())
	select {
	case err := <-result:
		require.ErrorContains(t, err, "closed")
	case <-time.After(time.Second):
		t.Fatal("lease did not wake on close")
	}
	batch.Release()
}
