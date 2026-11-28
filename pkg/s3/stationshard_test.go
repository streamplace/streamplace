//go:build stationslate && linux && cgo

package s3_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationshard"
)

const (
	stationshardDID = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	stationshardTTL = 2 * time.Second
)

type stationshardLive struct {
	cfg    s3.Config
	prefix string
	redact *strings.Replacer
}

func newStationshardLive(t *testing.T) (context.Context, stationshardLive) {
	t.Helper()
	cfg := stationtest.Config(t)
	f := stationshardLive{
		cfg:    cfg,
		prefix: "stationshard-live/" + uuid.NewString(),
		redact: strings.NewReplacer(cfg.SecretAccessKey, "[redacted]", cfg.AccessKeyID, "[redacted]"),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { f.cleanup(t) })
	return ctx, f
}

func (f stationshardLive) errorText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return f.redact.Replace(err.Error())
}

func (f stationshardLive) check(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %s; want %v", f.errorText(err), want)
	}
}

func (f stationshardLive) cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := s3.NewClient(f.cfg)
	prefix := f.prefix + "/"
	bucket := &f.cfg.Bucket
	pages := awss3.NewListObjectsV2Paginator(client, &awss3.ListObjectsV2Input{Bucket: bucket, Prefix: &prefix})
	var keys []string
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			t.Errorf("list test prefix: %s", f.errorText(err))
			break
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if !strings.HasPrefix(key, prefix) {
				t.Error("S3 listed an object outside the test prefix; refusing to delete it")
				continue
			}
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		_, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: bucket, Key: &key})
		if err != nil {
			t.Errorf("delete test object: %s", f.errorText(err))
		}
	}
}

func (f stationshardLive) open(t *testing.T, owner string, ttl time.Duration) *stationshard.Store {
	t.Helper()
	store, err := stationshard.Open(stationshard.Config{S3: f.cfg, Prefix: f.prefix, Owner: owner, TTL: ttl})
	f.check(t, err, nil)
	t.Cleanup(func() { f.check(t, store.Close(), nil) })
	return store
}

func waitForStationshard(t *testing.T, ctx context.Context, ready func() bool) {
	t.Helper()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for station lease to expire")
		case <-tick.C:
		}
	}
}

func parallelStationshard(n int, run func(int)) {
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range n {
		workers.Go(func() {
			<-start
			run(i)
		})
	}
	close(start)
	workers.Wait()
}

func TestStationshardLiveConditionalWrites(t *testing.T) {
	ctx, f := newStationshardLive(t)
	client := s3.NewClient(f.cfg)
	bucket, key := f.cfg.Bucket, f.prefix+"/conditional"
	create := func(value string) (*awss3.PutObjectOutput, error) {
		return client.PutObject(ctx, &awss3.PutObjectInput{
			Bucket: &bucket, Key: &key, IfNoneMatch: aws.String("*"), Body: strings.NewReader(value),
		})
	}
	requireRejected := func(condition string, err error) {
		t.Helper()
		var response interface{ HTTPStatusCode() int }
		require.True(t, errors.As(err, &response), "%s: expected S3 to reject the write, got %s", condition, f.errorText(err))
		require.Contains(t, []int{http.StatusPreconditionFailed, http.StatusConflict}, response.HTTPStatusCode(), condition)
	}
	read := func(want string) {
		t.Helper()
		out, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &bucket, Key: &key})
		f.check(t, err, nil)
		body, err := io.ReadAll(out.Body)
		closeErr := out.Body.Close()
		f.check(t, err, nil)
		f.check(t, closeErr, nil)
		require.Equal(t, want, string(body))
	}

	first, err := create("initial")
	f.check(t, err, nil)
	require.NotEmpty(t, aws.ToString(first.ETag))
	_, err = create("create must not overwrite")
	requireRejected("If-None-Match: *", err)
	read("initial")
	_, err = client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: &bucket, Key: &key, IfMatch: first.ETag, Body: strings.NewReader("updated"),
	})
	f.check(t, err, nil)
	_, err = client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: &bucket, Key: &key, IfMatch: first.ETag, Body: strings.NewReader("stale must not overwrite"),
	})
	requireRejected("stale If-Match", err)
	read("updated")
}

func TestStationshardLiveWALTakeover(t *testing.T) {
	ctx, f := newStationshardLive(t)
	did, key := stationshardDID, "wal"
	a := f.open(t, "node-a", stationshardTTL)
	var missing bool
	f.check(t, a.Update(ctx, did, key, func(old []byte) ([]byte, error) {
		missing = old == nil
		return []byte("first\n"), nil
	}), nil)
	require.True(t, missing, "initial update must see nil for a missing key")

	entered := make(chan string, 1)
	resume := make(chan struct{})
	done := make(chan struct{})
	var staleErr error
	go func() {
		defer close(done)
		var once sync.Once
		staleErr = a.Update(ctx, did, key, func(old []byte) ([]byte, error) {
			once.Do(func() { entered <- string(old) })
			<-resume
			return append(old, "stale\n"...), nil
		})
	}()
	resumeA := sync.OnceFunc(func() { close(resume) })
	defer func() {
		resumeA()
		<-done
	}()
	select {
	case before := <-entered:
		require.Equal(t, "first\n", before)
	case <-done:
		t.Fatalf("A.Update returned before its callback: %s", f.errorText(staleErr))
	case <-ctx.Done():
		t.Fatal("timed out waiting for A.Update to enter its callback")
	}

	b := f.open(t, "node-b", time.Minute)
	waitForStationshard(t, ctx, func() bool {
		owner, err := b.Owner(ctx, did)
		if errors.Is(err, stationshard.ErrNotFound) {
			return true
		}
		f.check(t, err, nil)
		require.Equal(t, "node-a", owner)
		return false
	})
	f.check(t, b.Update(ctx, did, key, func(old []byte) ([]byte, error) {
		return append(old, "takeover\n"...), nil
	}), nil)
	owner, err := b.Owner(ctx, did)
	f.check(t, err, nil)
	require.Equal(t, "node-b", owner)
	got, err := b.Get(ctx, did, key)
	f.check(t, err, nil)
	require.Equal(t, "first\ntakeover\n", string(got), "B must replay A's acknowledged WAL before writing")

	resumeA()
	<-done
	f.check(t, staleErr, stationshard.ErrFenced)
	got, err = b.Get(ctx, did, key)
	f.check(t, err, nil)
	require.Equal(t, "first\ntakeover\n", string(got))
}

func TestStationshardLiveRoleRestart(t *testing.T) {
	ctx, f := newStationshardLive(t)
	did, role, label := stationshardDID, "indexer", "same-node-label"
	a := f.open(t, label, stationshardTTL)
	_, err := a.ReadRole(ctx, did, role)
	f.check(t, err, stationshard.ErrNotFound)
	oldLease, err := a.ClaimRole(ctx, did, role)
	f.check(t, err, nil)
	f.check(t, a.UpdateRole(ctx, oldLease, func([]byte) ([]byte, error) {
		return []byte("before\n"), nil
	}), nil)

	b := f.open(t, label, time.Minute)
	var lease stationshard.Lease
	waitForStationshard(t, ctx, func() bool {
		var err error
		lease, err = b.ClaimRole(ctx, did, role)
		if errors.Is(err, stationshard.ErrHeld) {
			return false
		}
		f.check(t, err, nil)
		return true
	})
	require.Equal(t, label, lease.Owner)
	require.NotEmpty(t, lease.Incarnation)
	require.NotEqual(t, oldLease.Incarnation, lease.Incarnation)
	require.Greater(t, lease.Epoch, oldLease.Epoch)
	f.check(t, a.RenewRole(ctx, oldLease), stationshard.ErrFenced)
	f.check(t, a.ReleaseRole(ctx, oldLease), stationshard.ErrFenced)
	f.check(t, a.UpdateRole(ctx, oldLease, func([]byte) ([]byte, error) {
		return []byte("stale\n"), nil
	}), stationshard.ErrFenced)

	entries := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	updateErrors := make([]error, len(entries))
	parallelStationshard(len(entries), func(i int) {
		updateErrors[i] = b.UpdateRole(ctx, lease, func(old []byte) ([]byte, error) {
			return append(append(old, entries[i]...), '\n'), nil
		})
	})
	for _, err := range updateErrors {
		f.check(t, err, nil)
	}
	payload, active, err := b.ReadRoleState(ctx, did, role)
	f.check(t, err, nil)
	require.ElementsMatch(t, append(entries, "before"), strings.Split(strings.TrimSuffix(string(payload), "\n"), "\n"))
	require.Equal(t, lease, active)
	f.check(t, b.ReleaseRole(ctx, lease), nil)
	closed, active, err := a.ReadRoleState(ctx, did, role)
	f.check(t, err, nil)
	require.Equal(t, stationshard.Lease{}, active)
	require.Equal(t, payload, closed)
}

func TestStationshardLiveConcurrentClaims(t *testing.T) {
	ctx, f := newStationshardLive(t)
	nodes := []string{"node-a", "node-b"}
	stores := []*stationshard.Store{f.open(t, nodes[0], time.Minute), f.open(t, nodes[1], time.Minute)}
	dids := []string{stationshardDID, "did:plc:bbbbbbbbbbbbbbbbbbbbbanq"}
	require.Equal(t, stationshard.Slot(dids[0]), stationshard.Slot(dids[1]), "test DIDs must share a slot")
	_, err := stores[0].Owner(ctx, dids[0])
	f.check(t, err, stationshard.ErrNotFound)

	race := func(claim func(int) error) int {
		t.Helper()
		results := make([]error, len(stores))
		parallelStationshard(len(stores), func(i int) { results[i] = claim(i) })
		winner := -1
		for i, err := range results {
			if err != nil {
				f.check(t, err, stationshard.ErrHeld)
				continue
			}
			require.Equal(t, -1, winner, "both contenders claimed the same resource")
			winner = i
		}
		require.NotEqual(t, -1, winner, "neither contender claimed the resource")
		return winner
	}

	slotWinner := race(func(i int) error {
		return stores[i].Update(ctx, dids[i], "claim", func([]byte) ([]byte, error) {
			return []byte(nodes[i]), nil
		})
	})
	for i, store := range stores {
		owner, err := store.Owner(ctx, dids[i])
		f.check(t, err, nil)
		require.Equal(t, nodes[slotWinner], owner)
		routed, err := store.Route(ctx, dids[i])
		f.check(t, err, nil)
		require.Equal(t, owner, routed)
	}

	const role = "indexer"
	roleWinner := race(func(i int) error {
		_, err := stores[i].ClaimRole(ctx, dids[0], role)
		return err
	})
	_, err = stores[roleWinner].ClaimRole(ctx, dids[0], role)
	f.check(t, err, stationshard.ErrHeld)
}

func TestStationshardLiveImmutable(t *testing.T) {
	ctx, f := newStationshardLive(t)
	a := f.open(t, "node-a", time.Minute)
	b := f.open(t, "node-b", time.Minute)
	did, key := stationshardDID, "immutable"
	value := []byte("first value")
	_, err := b.GetImmutable(ctx, did, key)
	f.check(t, err, stationshard.ErrNotFound)
	f.check(t, a.PutImmutable(ctx, did, key, value), nil)
	f.check(t, b.PutImmutable(ctx, did, key, value), nil)
	f.check(t, b.PutImmutable(ctx, did, key, []byte("different value")), stationshard.ErrConflict)
	got, err := b.GetImmutable(ctx, did, key)
	f.check(t, err, nil)
	require.Equal(t, value, got)
}
