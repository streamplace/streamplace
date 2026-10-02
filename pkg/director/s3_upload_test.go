package director

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// A recording applies uploads and cutovers in arrival order, even when an
// earlier segment's archive copy takes longer to prepare than later ones, and
// still applies the ones queued when the session ended.
func TestS3OperationsKeepArrivalOrder(t *testing.T) {
	ss := &StreamSession{started: make(chan struct{}), g: &errgroup.Group{}}
	close(ss.started)
	var mu sync.Mutex
	var order []string
	var errs []error
	record := func(ctx context.Context, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, string(data))
		errs = append(errs, ctx.Err())
		return nil
	}
	prepare := func(name string, delay time.Duration) func(context.Context) []byte {
		return func(context.Context) []byte {
			time.Sleep(delay)
			return []byte(name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	ss.s3InOrder(ctx, prepare("first", 50*time.Millisecond), record)
	ss.s3InOrder(ctx, nil, func(ctx context.Context, _ []byte) error { return record(ctx, []byte("cutover")) })
	ss.s3InOrder(ctx, prepare("third", 0), record)
	cancel() // the session ends while the operations are still queued
	require.NoError(t, ss.g.Wait())
	require.Equal(t, []string{"first", "cutover", "third"}, order)
	require.Equal(t, []error{nil, nil, nil}, errs, "queued operations reach the uploader after the session ends")
}
