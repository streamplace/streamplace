package stt

import (
	"context"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEngineProxySharesParentBudgetAndReleasesDisconnectedWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	engine := fakeScheduler(1)
	defer engine.Close()
	path := filepath.Join(t.TempDir(), "engine.sock")
	stop, err := ServeEngine(ctx, path, engine)
	require.NoError(t, err)
	defer stop()
	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
	parent, err := engine.Lease(ctx, LeaseOptions{Realtime: true, Model: "tiny"})
	require.NoError(t, err)
	defer parent.Release()
	worker, err := NewProxy(path).Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	require.Equal(t, "tiny", worker.Model().Info().Name)
	_, err = NewProxy(path).Lease(ctx, LeaseOptions{Realtime: true})
	require.ErrorIs(t, err, ErrOverBudget)
	// Simulate SIGKILL: only the socket disappears; Release never runs in worker.
	require.NoError(t, worker.(*proxyLease).conn.Close())
	require.Eventually(t, func() bool { engine.mu.Lock(); defer engine.mu.Unlock(); return len(engine.leases) == 1 }, time.Second, time.Millisecond)
	proxy := NewProxy(path)
	replacement, err := proxy.Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	require.NoError(t, proxy.Close())
	require.Nil(t, replacement.Model())
	require.Eventually(t, func() bool { engine.mu.Lock(); defer engine.mu.Unlock(); return len(engine.leases) == 1 }, time.Second, time.Millisecond)
}
func TestEngineProxyCancelsWaitingLeaseAndBoundsFrames(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	engine := fakeScheduler(2)
	defer engine.Close()
	path := filepath.Join(t.TempDir(), "engine.sock")
	stop, err := ServeEngine(ctx, path, engine)
	require.NoError(t, err)
	defer stop()
	parent, err := engine.Lease(ctx, LeaseOptions{})
	require.NoError(t, err)
	defer parent.Release()
	short, stopShort := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stopShort()
	_, err = NewProxy(path).Lease(short, LeaseOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	c, err := net.Dial("unix", path)
	require.NoError(t, err)
	defer c.Close()
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], engineFrameLimit+1)
	_, err = c.Write(header[:])
	require.NoError(t, err)
	require.NoError(t, c.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = c.Read(header[:])
	require.Error(t, err)
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded)
}

type cancelProxyModel struct {
	entered  chan struct{}
	canceled chan struct{}
}

func (m *cancelProxyModel) Info() ModelInfo { return ModelInfo{Name: "cancel-model"} }
func (m *cancelProxyModel) Transcribe(ctx context.Context, _ []float32, _ Options) (*Result, error) {
	close(m.entered)
	<-ctx.Done()
	close(m.canceled)
	return nil, ctx.Err()
}

func TestEngineProxyWorkerCancellationStopsParentInference(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	engine := fakeScheduler(1)
	defer engine.Close()
	model := &cancelProxyModel{entered: make(chan struct{}), canceled: make(chan struct{})}
	engine.models = []scheduledModel{{model: model, cost: .5}}
	path := filepath.Join(t.TempDir(), "engine.sock")
	stop, err := ServeEngine(ctx, path, engine)
	require.NoError(t, err)
	defer stop()
	lease, err := NewProxy(path).Lease(ctx, LeaseOptions{Realtime: true})
	require.NoError(t, err)
	defer lease.Release()
	inference, stopInference := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := lease.Model().Transcribe(inference, make([]float32, SampleRate), Options{})
		done <- err
	}()
	select {
	case <-model.entered:
	case <-ctx.Done():
		t.Fatal("parent inference never entered")
	}
	stopInference()
	require.ErrorIs(t, <-done, context.Canceled)
	select {
	case <-model.canceled:
	case <-ctx.Done():
		t.Fatal("parent inference did not observe worker cancellation")
	}
	require.Eventually(t, func() bool { engine.mu.Lock(); defer engine.mu.Unlock(); return len(engine.leases) == 0 }, time.Second, time.Millisecond)
}
