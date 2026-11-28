//go:build linux && cgo && stationslate

package statedb

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationdb"
)

type stationHeadResponseLost struct{}

// Pause B's conditional PUT after it has built its commit, so A can change
// the real S3 head before B attempts to publish it.
func stationContentionPair(t *testing.T) (a, b *StatefulDB, pending <-chan chan<- bool) {
	t.Helper()
	bucket := stationtest.Config(t)
	prefix := "station-contention-" + uuid.NewString()
	var key [32]byte
	rand.Read(key[:])
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	open := func(cfg s3.Config) *StatefulDB {
		t.Helper()
		db, store, err := openStationDB(ctx, cfg, prefix, key[:])
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return &StatefulDB{DB: db.WithContext(ctx), stationStore: store}
	}
	a = open(bucket)
	require.NoError(t, a.DB.AutoMigrate(&Repo{}, &oatproxy.OAuthSession{}))

	endpoint, err := url.Parse(bucket.Endpoint)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(endpoint)
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusOK && resp.Request.Context().Value(stationHeadResponseLost{}) != nil {
			return errors.New("lost station head PUT acknowledgement")
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusBadGateway)
	}
	attempts := make(chan chan<- bool)
	head := "/" + bucket.Bucket + "/" + prefix + "/stationdb/v1/head"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == head {
			resume := make(chan bool, 1)
			select {
			case attempts <- resume:
			case <-ctx.Done():
				return
			}
			select {
			case lost := <-resume:
				if lost {
					r = r.WithContext(context.WithValue(r.Context(), stationHeadResponseLost{}, true))
				}
			case <-ctx.Done():
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	bucket.Endpoint = server.URL
	return a, open(bucket), attempts
}

func stationWriteAsync(write func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- write() }()
	return done
}

func stationHeadAttempt(t *testing.T, pending <-chan chan<- bool, done <-chan error) chan<- bool {
	t.Helper()
	select {
	case resume := <-pending:
		return resume
	case err := <-done:
		t.Fatalf("station write returned before publishing: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("station write did not reach head PUT")
	}
	return nil
}

func stationWriteResult(t *testing.T, pending <-chan chan<- bool, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case resume := <-pending:
		resume <- false
		t.Fatal("unexpected extra attempt to publish a station write")
	case <-time.After(5 * time.Second):
		t.Fatal("station write did not return")
	}
	return nil
}

func TestStationQueueEffectRetriesRejectedCommits(t *testing.T) {
	a, b, pending := stationContentionPair(t)
	ctx := t.Context()
	require.NoError(t, a.DB.AutoMigrate(&AppTask{}, &AppTaskEffect{}))
	task := enqueueClaimedTask(t, a, "push-conflict", nil)
	claim := func(state *StatefulDB) context.Context {
		return context.WithValue(ctx, queueClaimKey{}, queueClaim{state, task.ID, task.ClaimToken})
	}

	var dispatches atomic.Int32
	send := func(context.Context) (bool, error) {
		dispatches.Add(1)
		return true, nil
	}
	done := stationWriteAsync(func() error {
		_, err := RunTaskEffect(claim(b), "push-b", send)
		return err
	})
	// Advance the head while B's reservation and then its receipt await CAS.
	for _, winner := range []string{"push-a1", "push-a2"} {
		reject := stationHeadAttempt(t, pending, done)
		_, err := RunTaskEffect(claim(a), winner, send)
		require.NoError(t, err)
		reject <- false
		stationHeadAttempt(t, pending, done) <- false
	}
	require.NoError(t, stationWriteResult(t, pending, done))
	require.EqualValues(t, 3, dispatches.Load())
	for _, key := range []string{"push-b", "push-a1", "push-a2"} {
		receipt, found, err := ReadTaskEffect[bool](claim(a), key)
		require.NoError(t, err)
		require.True(t, found && receipt, "missing delivery receipt for %s", key)
	}
	require.NoError(t, a.CompleteTask(ctx, task.ID, task.ClaimToken))
	requireTaskStatus(t, a, task.ID, TaskStatusCompleted)
}

func TestStationRepoBootstrapContention(t *testing.T) {
	a, b, pending := stationContentionPair(t)
	const first, second = "did:plc:first", "did:plc:second"
	done := stationWriteAsync(func() error { return b.AddRepo(first) })
	resume := stationHeadAttempt(t, pending, done)
	require.NoError(t, a.AddRepo(first))
	before, err := a.ListRepos(10, 0)
	require.NoError(t, err)
	require.Len(t, before, 1)
	resume <- false
	require.NoError(t, stationWriteResult(t, pending, done))
	got, err := b.ListRepos(10, 0)
	require.NoError(t, err)
	require.Equal(t, before, got, "same-DID contention must not replace the first registration")

	done = stationWriteAsync(func() error { return b.AddRepo(second) })
	resume = stationHeadAttempt(t, pending, done)
	const third = "did:plc:third"
	require.NoError(t, a.AddRepo(third))
	resume <- false
	stationHeadAttempt(t, pending, done) <- false
	require.NoError(t, stationWriteResult(t, pending, done))
	got, err = a.ListRepos(10, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.ElementsMatch(t, []string{first, second, third}, []string{got[0].DID, got[1].DID, got[2].DID})
}

func TestStationOAuthContention(t *testing.T) {
	for _, name := range []string{"distinct", "replay", "proof-after-revoke", "revoke-after-proof", "ambiguous"} {
		t.Run(name, func(t *testing.T) {
			a, b, pending := stationContentionPair(t)
			const id = "station-contention"
			require.NoError(t, a.CreateOAuthSession(id, &oatproxy.OAuthSession{
				DownstreamDPoPJKT: id, DownstreamAccessToken: "downstream-token",
			}))
			left := loadStationTestOAuthSession(t, a, id)
			right := loadStationTestOAuthSession(t, b, id)
			revokedAt := time.Now().UTC()
			if name == "revoke-after-proof" {
				right.DownstreamRevokedAt = &revokedAt
			} else {
				require.NoError(t, right.CacheJTI("right"))
			}
			submitted := *right
			done := stationWriteAsync(func() error { return b.UpdateOAuthSession(id, right) })
			resume := stationHeadAttempt(t, pending, done)
			switch name {
			case "proof-after-revoke":
				left.DownstreamRevokedAt = &revokedAt
			case "replay":
				require.NoError(t, left.CacheJTI("right"))
			case "distinct", "revoke-after-proof":
				require.NoError(t, left.CacheJTI("left"))
			}
			if name != "ambiguous" {
				require.NoError(t, a.UpdateOAuthSession(id, left))
			}
			resume <- name == "ambiguous"
			if name == "distinct" {
				resume = stationHeadAttempt(t, pending, done)
				require.NoError(t, a.AddRepo("did:plc:concurrent"))
				resume <- false
			}
			if name == "distinct" || name == "revoke-after-proof" {
				stationHeadAttempt(t, pending, done) <- false
			}
			err := stationWriteResult(t, pending, done)
			after := loadStationTestOAuthSession(t, a, id)
			switch name {
			case "replay", "proof-after-revoke":
				require.ErrorIs(t, err, stationdb.ErrConflict)
				require.Equal(t, submitted, *right)
				require.Equal(t, *left, *after)
			case "ambiguous":
				require.ErrorIs(t, err, stationdb.ErrCommitUnknown)
				require.Equal(t, submitted, *right)
				require.ErrorContains(t, after.CacheJTI("right"), "jti already used")
			case "distinct":
				require.NoError(t, err)
				require.Equal(t, *right, *after)
				require.ErrorContains(t, after.CacheJTI("left"), "jti already used")
				require.ErrorContains(t, after.CacheJTI("right"), "jti already used")
			case "revoke-after-proof":
				require.NoError(t, err)
				require.Equal(t, *right, *after)
				require.Equal(t, oatproxy.OAuthSessionStateReadyUpstream, after.Status())
				require.ErrorContains(t, after.CacheJTI("left"), "jti already used")
			}
		})
	}
}
