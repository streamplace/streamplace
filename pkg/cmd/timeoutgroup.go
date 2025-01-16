package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
	"golang.org/x/sync/errgroup"
)

func TimeoutGroupWithContext(ctx context.Context) (*TimeoutGroup, context.Context) {
	group, ctx2 := errgroup.WithContext(ctx)
	tg := &TimeoutGroup{
		ErrGroup: group,
		exitCh:   make(chan error),
	}
	return tg, ctx2
}

// errgroup wrapper that self-destructs if things aren't shutting down properly
type TimeoutGroup struct {
	ErrGroup       *errgroup.Group
	selfDestructMu sync.Mutex
	exitCh         chan error
}

func (g *TimeoutGroup) Go(f func() error) {
	g.ErrGroup.Go(func() error {
		err := f()
		g.selfDestruct(err)
		return err
	})
}

func (g *TimeoutGroup) Wait() error {
	go func() {
		g.exitCh <- g.ErrGroup.Wait()
	}()
	return <-g.exitCh
}

func (g *TimeoutGroup) selfDestruct(err error) {
	first := g.selfDestructMu.TryLock()
	if !first {
		return
	}
	go func() {
		log.Log(context.Background(), "app terminating", "reason", err)
		time.Sleep(5 * time.Second)
		g.exitCh <- fmt.Errorf("selfDestruct terminated app after timeout, reason for shutdown: %w", err)
	}()
}
