package statedb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"stream.place/streamplace/pkg/appbsky"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
)

func dequeueTestTask(t *testing.T, state *StatefulDB, kind string) *AppTask {
	t.Helper()
	task, err := state.DequeueTask(t.Context(), "test-worker", kind)
	require.NoError(t, err)
	require.NotNil(t, task)
	return task
}

func enqueueClaimedTask(t *testing.T, state *StatefulDB, kind string, payload any, opts ...TaskOption) *AppTask {
	t.Helper()
	enqueued, err := state.EnqueueTask(t.Context(), kind, payload, opts...)
	require.NoError(t, err)
	claimed := dequeueTestTask(t, state, kind)
	require.Equal(t, enqueued.ID, claimed.ID)
	return claimed
}

func requireTaskStatus(t *testing.T, state *StatefulDB, id uint, status TaskStatus) *AppTask {
	t.Helper()
	task, err := state.GetTask(t.Context(), id)
	require.NoError(t, err)
	require.NotNil(t, task)
	require.Equal(t, status, task.Status)
	if status != TaskStatusProcessing {
		require.Empty(t, task.ClaimToken)
		require.Nil(t, task.LockExpires)
		require.Nil(t, task.WorkerID)
	}
	return task
}

func expireTestTask(t *testing.T, state *StatefulDB, task *AppTask) {
	t.Helper()
	res := state.DB.Model(&AppTask{}).Where("id = ? AND claim_token = ?", task.ID, task.ClaimToken).
		UpdateColumn("lock_expires", time.Now().Add(-time.Minute).UTC())
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)
}

func TestTaskClaimsFenceExpiredAndReusedWorker(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		first := enqueueClaimedTask(t, state, "fenced", nil, WithMaxTries(2))
		require.Equal(t, 1, first.TryCount)
		require.NotEmpty(t, first.ClaimToken)
		require.NotNil(t, first.LockExpires)
		stale := func() {
			require.ErrorIs(t, state.RenewTask(ctx, first.ID, first.ClaimToken), ErrTaskClaimLost)
			require.ErrorIs(t, state.ReleaseTask(ctx, first.ID, first.ClaimToken), ErrTaskClaimLost)
			require.ErrorIs(t, state.FailTask(ctx, first.ID, first.ClaimToken, "stale"), ErrTaskClaimLost)
			require.ErrorIs(t, state.CompleteTask(ctx, first.ID, first.ClaimToken), ErrTaskClaimLost)
		}
		expireTestTask(t, state, first)
		stale() // an expired claim cannot revive or mutate even before takeover
		next := dequeueTestTask(t, state, first.Type)
		require.Equal(t, first.ID, next.ID)
		require.Equal(t, first.WorkerID, next.WorkerID)
		require.NotEqual(t, first.ClaimToken, next.ClaimToken)
		require.Equal(t, 2, next.TryCount)
		stale()
		require.ErrorIs(t, state.CompleteTask(ctx, first.ID, ""), ErrTaskClaimLost)
		current := requireTaskStatus(t, state, next.ID, TaskStatusProcessing)
		require.Equal(t, next.ClaimToken, current.ClaimToken)
		require.NoError(t, state.RenewTask(ctx, current.ID, current.ClaimToken))
		require.NoError(t, state.CompleteTask(ctx, current.ID, current.ClaimToken))
		require.ErrorIs(t, state.CompleteTask(ctx, current.ID, current.ClaimToken), ErrTaskClaimLost)
		requireTaskStatus(t, state, next.ID, TaskStatusCompleted)
	})
}

func TestTaskAttemptsBoundedOnFailureReleaseAndTimeout(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		first := enqueueClaimedTask(t, state, "bounded", nil, WithMaxTries(3))
		require.NoError(t, state.FailTask(ctx, first.ID, first.ClaimToken, "retry"))
		second := dequeueTestTask(t, state, first.Type)
		require.NoError(t, state.ReleaseTask(ctx, second.ID, second.ClaimToken))
		third := dequeueTestTask(t, state, first.Type)
		require.Equal(t, 3, third.TryCount)
		expireTestTask(t, state, third)
		next, err := state.DequeueTask(ctx, "test-worker", first.Type)
		require.NoError(t, err)
		require.Nil(t, next)
		failed := requireTaskStatus(t, state, third.ID, TaskStatusFailed)
		require.Equal(t, ptr("task claim expired"), failed.Error)

		task := enqueueClaimedTask(t, state, "last-release", nil, WithMaxTries(1))
		require.NoError(t, state.ReleaseTask(ctx, task.ID, task.ClaimToken))
		requireTaskStatus(t, state, task.ID, TaskStatusFailed)

		task = enqueueClaimedTask(t, state, "last-unconfirmed", nil, WithMaxTries(1))
		_, err = state.beginTaskEffect(ctx, task.ID, task.ClaimToken, "unconfirmed", "")
		require.NoError(t, err)
		expireTestTask(t, state, task)
		cleaned, err := state.CleanupExpiredLocks(ctx)
		require.NoError(t, err)
		require.EqualValues(t, 1, cleaned)
		requireTaskStatus(t, state, task.ID, TaskStatusBlocked)
	})
}

func TestConcurrentTaskDedupAndClaim(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		const count = 12
		type result struct {
			task *AppTask
			err  error
		}
		parallel := func(fn func() (*AppTask, error)) []result {
			var wg sync.WaitGroup
			start := make(chan struct{})
			results := make([]result, count)
			for i := range results {
				wg.Go(func() {
					<-start
					results[i].task, results[i].err = fn()
				})
			}
			close(start)
			wg.Wait()
			return results
		}
		queued := parallel(func() (*AppTask, error) {
			return state.EnqueueTask(ctx, "race", nil, WithTaskKey("one-key"))
		})
		for _, r := range queued {
			require.NoError(t, r.err)
			require.NotNil(t, r.task)
			require.Equal(t, queued[0].task.ID, r.task.ID)
		}
		claimed := parallel(func() (*AppTask, error) {
			return state.DequeueTask(ctx, "same-worker", "race")
		})
		var winner *AppTask
		for _, r := range claimed {
			require.NoError(t, r.err)
			if r.task != nil {
				require.Nil(t, winner, "only one concurrent claim")
				winner = r.task
			}
		}
		require.NotNil(t, winner)
		require.Equal(t, queued[0].task.ID, winner.ID)
		require.Equal(t, 1, winner.TryCount)
		require.NoError(t, state.CompleteTask(ctx, winner.ID, winner.ClaimToken))
	})
}

func TestTaskEffectReceiptRaceWithAcknowledgement(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		for range 6 {
			task := enqueueClaimedTask(t, state, "effect-race", nil)
			_, err := state.beginTaskEffect(ctx, task.ID, task.ClaimToken, "record", "")
			require.NoError(t, err)
			start := make(chan struct{})
			saved := make(chan error, 1)
			acked := make(chan error, 1)
			go func() {
				<-start
				saved <- state.setTaskEffect(ctx, task.ID, task.ClaimToken, "record", []byte("true"))
			}()
			go func() {
				<-start
				acked <- state.CompleteTask(ctx, task.ID, task.ClaimToken)
			}()
			close(start)
			saveErr, ackErr := <-saved, <-acked
			if saveErr == nil {
				require.NoError(t, ackErr, "an acknowledgement after a committed receipt cannot block")
				requireTaskStatus(t, state, task.ID, TaskStatusCompleted)
			} else {
				require.ErrorIs(t, saveErr, ErrTaskClaimLost)
				require.ErrorIs(t, ackErr, ErrTaskEffectUncertain)
				requireTaskStatus(t, state, task.ID, TaskStatusBlocked)
			}
		}
	})
}

type taskEffectReadRows struct {
	logger.Interface
	maxRows int64
}

func (l *taskEffectReadRows) Trace(_ context.Context, _ time.Time, query func() (string, int64), _ error) {
	sql, rows := query()
	if strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, "app_task_effects") {
		l.maxRows = max(l.maxRows, rows)
	}
}

func TestTaskEffectLookupWithLargeReceiptsAndTakeover(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		first := enqueueClaimedTask(t, state, "large-effects", nil)
		large := append([]byte{'"'}, bytes.Repeat([]byte{'x'}, 32<<10)...)
		large = append(large, '"')
		rows := make([]AppTaskEffect, 96)
		for i := range rows {
			rows[i] = AppTaskEffect{TaskID: first.ID, EffectKey: fmt.Sprintf("prior-%d", i), Receipt: large}
		}
		require.NoError(t, state.DB.CreateInBatches(&rows, 32).Error)
		const target = "at://did:web:creator.example/place.stream.video/original-rkey"
		_, err := state.beginTaskEffect(ctx, first.ID, first.ClaimToken, "confirmed", "")
		require.NoError(t, err)
		require.NoError(t, state.setTaskEffect(ctx, first.ID, first.ClaimToken, "confirmed", []byte("null")))
		_, err = state.beginTaskEffect(ctx, first.ID, first.ClaimToken, "reserved", target)
		require.NoError(t, err)
		expireTestTask(t, state, first)
		next := dequeueTestTask(t, state, first.Type)
		require.NotEqual(t, first.ClaimToken, next.ClaimToken)

		log := &taskEffectReadRows{Interface: state.DB.Logger}
		point := &StatefulDB{DB: state.DB.Session(&gorm.Session{Logger: log}), Type: state.Type}
		_, err = point.readTaskEffect(ctx, first.ID, first.ClaimToken, "confirmed")
		require.ErrorIs(t, err, ErrTaskClaimLost)
		require.ErrorIs(t, point.setTaskEffect(ctx, first.ID, first.ClaimToken, "reserved", []byte("true")), ErrTaskClaimLost)
		prior, err := point.readTaskEffect(ctx, next.ID, next.ClaimToken, "confirmed")
		require.NoError(t, err)
		require.Equal(t, []byte("null"), prior, "JSON null is a confirmed receipt, not SQL NULL")
		_, err = point.readTaskEffect(ctx, next.ID, next.ClaimToken, "reserved")
		require.ErrorIs(t, err, ErrTaskEffectUncertain)
		_, err = point.beginTaskEffect(ctx, next.ID, next.ClaimToken, "reserved", target+"-different")
		require.ErrorIs(t, err, ErrTaskEffectUncertain)
		missing, err := point.readTaskEffect(ctx, next.ID, next.ClaimToken, "missing")
		require.NoError(t, err)
		require.Nil(t, missing)
		exists, err := point.hasTaskEffects(ctx, next.ID, next.ClaimToken)
		require.NoError(t, err)
		require.True(t, exists)
		require.ErrorIs(t, point.CompleteTask(ctx, next.ID, next.ClaimToken), ErrTaskEffectUncertain)
		require.LessOrEqual(t, log.maxRows, int64(1), "point queries must not return other targets' receipts")
		var reserved AppTaskEffect
		require.NoError(t, state.DB.Where("task_id = ? AND effect_key = ?", next.ID, "reserved").Take(&reserved).Error)
		require.Equal(t, target, reserved.Target)
		require.Nil(t, reserved.Receipt)
	})
}

func TestTaskHeartbeatRenewsCancelsAndFencesDispatch(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		task := enqueueClaimedTask(t, state, TaskViewCountAggregate, ViewCountAggregateTask{})
		started := make(chan context.Context, 1)
		var dispatches atomic.Int32
		send := func(context.Context) (bool, error) {
			dispatches.Add(1)
			return true, nil
		}
		state.SetViewCountAggregator(func(ctx context.Context, _ ViewCountAggregateTask) error {
			started <- ctx
			<-ctx.Done()
			_, err := RunTaskEffect(ctx, "stale", send)
			return err
		})
		finished := make(chan error, 1)
		go func() { finished <- state.runClaimedTask(ctx, task, 10*time.Millisecond) }()
		var oldCtx context.Context
		select {
		case oldCtx = <-started:
		case <-ctx.Done():
			t.Fatal("handler did not start")
		}
		shortLease := time.Now().Add(time.Second).UTC()
		require.NoError(t, state.DB.Model(&AppTask{}).Where("id = ?", task.ID).UpdateColumn("lock_expires", shortLease).Error)
		require.Eventually(t, func() bool {
			row, err := state.GetTask(ctx, task.ID)
			return err == nil && row != nil && row.LockExpires != nil && row.LockExpires.After(shortLease)
		}, time.Second, 10*time.Millisecond)

		expireTestTask(t, state, task)
		next := dequeueTestTask(t, state, task.Type)
		_, err := RunTaskEffect(context.WithoutCancel(oldCtx), "stale", send)
		require.ErrorIs(t, err, ErrTaskClaimLost, "dispatch is fenced even before noticing cancellation")
		select {
		case err := <-finished:
			require.ErrorIs(t, err, ErrTaskClaimLost)
		case <-ctx.Done():
			t.Fatal("lost claim did not cancel handler")
		}
		require.Zero(t, dispatches.Load())
		require.NoError(t, state.CompleteTask(ctx, next.ID, next.ClaimToken))
	})
}

func TestQueueCloseWaitsForHandlerBeforeRelease(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		started, canceled, exit := make(chan struct{}), make(chan struct{}), make(chan struct{})
		defer close(exit)
		state.SetViewCountAggregator(func(ctx context.Context, _ ViewCountAggregateTask) error {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-exit
			return ctx.Err()
		})
		task, err := state.EnqueueTask(ctx, TaskViewCountAggregate, ViewCountAggregateTask{})
		require.NoError(t, err)
		finished := make(chan error, 1)
		go func() { finished <- state.ProcessQueue(ctx, 1) }()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not start")
		}
		cancel()
		select {
		case <-canceled:
		case <-time.After(5 * time.Second):
			t.Fatal("handler did not receive cancellation")
		}
		select {
		case err := <-finished:
			t.Fatalf("queue closed while handler was still running: %v", err)
		default:
		}
		requireTaskStatus(t, state, task.ID, TaskStatusProcessing)
		exit <- struct{}{}
		select {
		case err := <-finished:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("queue did not finish after handler exited")
		}
		pending := requireTaskStatus(t, state, task.ID, TaskStatusPending)
		require.Equal(t, 1, pending.TryCount)
	})
}

type queueNotifier func(context.Context, []notificationpkg.NotificationTarget, *notificationpkg.NotificationBlast) error

func (f queueNotifier) Blast(ctx context.Context, targets []notificationpkg.NotificationTarget, blast *notificationpkg.NotificationBlast) error {
	return f(ctx, targets, blast)
}

func TestNotificationTakeoverContinuesIndependentEffects(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		const did = "did:web:streamer.example"
		const follower = "did:web:follower.example"
		require.NoError(t, state.model.CreateFollow(ctx, follower, "rkey", appbsky.GraphFollow{
			Subject: did, CreatedAt: "2026-01-01T00:00:00Z",
		}))
		calls := map[string]int{"confirmed": 1, "crashed": 1}
		for _, token := range []string{"confirmed", "crashed", "ambiguous", "remaining"} {
			require.NoError(t, state.CreateNotification(token, follower, NotificationTypeFirebase))
		}
		state.SetNotifier(queueNotifier(func(_ context.Context, targets []notificationpkg.NotificationTarget, _ *notificationpkg.NotificationBlast) error {
			require.Len(t, targets, 1)
			token := targets[0].Token
			calls[token]++
			if token == "ambiguous" {
				return errors.New("provider lost response")
			}
			return nil
		}))
		first := enqueueClaimedTask(t, state, TaskNotification, NotificationTask{
			Livestream: placestream.Livestream_LivestreamView{
				Author: appbsky.ActorDefs_ProfileViewBasic{Did: did},
				Record: &glex.LexiconTypeDecoder{Val: &placestream.Livestream{Title: "Live"}},
			},
		})
		confirmed := taskEffectKey("push", "confirmed")
		_, err := state.beginTaskEffect(ctx, first.ID, first.ClaimToken, confirmed, "")
		require.NoError(t, err)
		require.NoError(t, state.setTaskEffect(ctx, first.ID, first.ClaimToken, confirmed, []byte("false")))
		crashed := taskEffectKey("push", "crashed")
		_, err = state.beginTaskEffect(ctx, first.ID, first.ClaimToken, crashed, "")
		require.NoError(t, err)
		expireTestTask(t, state, first)

		next := dequeueTestTask(t, state, first.Type)
		require.ErrorIs(t, state.runClaimedTask(ctx, next, time.Hour), ErrTaskEffectUncertain)
		requireTaskStatus(t, state, next.ID, TaskStatusBlocked)
		var unresolved AppTaskEffect
		require.NoError(t, state.DB.Where("task_id = ? AND effect_key = ?", next.ID, crashed).Take(&unresolved).Error)
		require.Nil(t, unresolved.Receipt)
		require.Equal(t, map[string]int{"confirmed": 1, "crashed": 1, "ambiguous": 1, "remaining": 1}, calls)
		noRetry, err := state.DequeueTask(ctx, "any-worker", first.Type)
		require.NoError(t, err)
		require.Nil(t, noRetry)
	})
}

func TestWebhookRetriesKnownRejectionButNotConfirmedDelivery(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		var retry, success atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/retry" {
				if retry.Add(1) == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
			} else {
				success.Add(1)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		const did = "did:web:streamer.example"
		for _, path := range []string{"/retry", "/success"} {
			require.NoError(t, state.CreateWebhook(&Webhook{UserDID: did, URL: server.URL + path, Active: true, Events: []byte(`["stream.received"]`)}))
		}
		task := enqueueClaimedTask(t, state, TaskStreamReceived, StreamReceivedTask{StreamerDID: did})
		require.Error(t, state.runClaimedTask(ctx, task, time.Hour))
		requireTaskStatus(t, state, task.ID, TaskStatusPending)
		next := dequeueTestTask(t, state, task.Type)
		require.NoError(t, state.runClaimedTask(ctx, next, time.Hour))
		requireTaskStatus(t, state, task.ID, TaskStatusCompleted)
		require.EqualValues(t, 2, retry.Load())
		require.EqualValues(t, 1, success.Load())
	})
}

func TestQueueCentralAckAndPanicFailure(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		viewRuns, ingestRuns := 0, 0
		state.SetViewCountAggregator(func(context.Context, ViewCountAggregateTask) error { viewRuns++; return nil })
		state.SetCDNLogIngester(func(context.Context) error { ingestRuns++; return nil })
		for _, test := range []struct {
			kind    string
			payload any
		}{
			{TaskChat, ChatTask{MessageView: placestream.ChatDefs_MessageView{
				Record: &glex.LexiconTypeDecoder{Val: &placestream.ChatMessage{Streamer: "did:web:streamer.example"}},
			}}},
			{TaskViewCountAggregate, ViewCountAggregateTask{}},
			{TaskCDNLogIngest, CDNLogIngestTask{}},
		} {
			task := enqueueClaimedTask(t, state, test.kind, test.payload)
			require.NoError(t, state.runClaimedTask(ctx, task, time.Hour))
			requireTaskStatus(t, state, task.ID, TaskStatusCompleted)
			require.ErrorIs(t, state.runClaimedTask(ctx, task, time.Hour), ErrTaskClaimLost)
		}
		require.Equal(t, 1, viewRuns)
		require.Equal(t, 1, ingestRuns)
		state.SetCDNLogIngester(func(context.Context) error { panic("ingester panic") })
		task := enqueueClaimedTask(t, state, TaskCDNLogIngest, CDNLogIngestTask{}, WithMaxTries(1))
		require.ErrorContains(t, state.runClaimedTask(ctx, task, time.Hour), "ingester panic")
		requireTaskStatus(t, state, task.ID, TaskStatusFailed)
	})
}
