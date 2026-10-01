package stt

import (
	"context"
	"testing"

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
	e := &scheduler{budget: budget, ready: ready, done: done, cancel: func() {}, leases: make(map[*engineLease]struct{})}
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
	first, err := e.Lease(ctx)
	require.NoError(t, err)
	second, err := e.Lease(ctx)
	require.NoError(t, err)
	require.Equal(t, "small", first.Model().Info().Name)
	third, err := e.Lease(ctx)
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
func TestSchedulerRefusesUnfitLeases(t *testing.T) {
	e := fakeScheduler(1)
	defer e.Close()
	first, err := e.Lease(context.Background())
	require.NoError(t, err)
	require.Equal(t, "base", first.Model().Info().Name)
	first.Release()
	a, err := e.Lease(context.Background())
	require.NoError(t, err)
	b, err := e.Lease(context.Background())
	require.NoError(t, err)
	require.Equal(t, "tiny", a.Model().Info().Name)
	require.Equal(t, "tiny", b.Model().Info().Name)
	_, err = e.Lease(context.Background())
	require.ErrorIs(t, err, ErrOverBudget)
}
