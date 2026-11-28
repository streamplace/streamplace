package upload_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/upload"
)

func TestStationUploadDurabilityAndClaimFencing(t *testing.T) {
	cfg := stationtest.Config(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	namespace := uuid.NewString()
	keyFile := filepath.Join(t.TempDir(), "station.key")
	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, key[:], 0o600))

	openState := func() (*statedb.StatefulDB, context.Context, func()) {
		cli := &config.CLI{DBURL: "station://" + namespace, DataDir: t.TempDir(), StationStateKeyFile: keyFile}
		cli.SetS3Config(cfg)
		stationCtx, stop := context.WithCancel(ctx)
		t.Cleanup(stop)
		state, err := statedb.MakeDB(stationCtx, cli, nil, nil)
		require.NoError(t, err)
		sqlDB, err := state.DB.DB()
		require.NoError(t, err)
		closeState := sync.OnceFunc(func() {
			stop()
			if err := sqlDB.Close(); err != nil {
				t.Errorf("close station DB: %v", err)
			}
		})
		t.Cleanup(closeState)
		return state, stationCtx, closeState
	}
	openWorker := func() (*statedb.StatefulDB, *upload.Manager, string, func()) {
		state, stationCtx, closeState := openState()
		mgr, err := upload.NewStation(stationCtx, state.CLI, state)
		require.NoError(t, err)
		server := httptest.NewUnstartedServer(mgr)
		server.Config.BaseContext = func(net.Listener) context.Context { return stationCtx }
		server.Start()
		closeWorker := sync.OnceFunc(func() {
			server.Close()
			closeState()
		})
		t.Cleanup(closeWorker)
		return state, mgr, server.URL, closeWorker
	}

	_, manager, firstURL, closeFirst := openWorker()
	_, _, secondURL, closeSecond := openWorker()
	const prefixSize = 64 << 10 // Immutable staging objects have no multipart minimum.
	prefixes := [2]string{strings.Repeat("a", prefixSize), strings.Repeat("b", prefixSize)}
	tail := "tail:" + namespace
	totalSize := prefixSize + len(tail)
	created, err := manager.Create(ctx, "did:plc:station-upload-test", "video/mp4", "station.mp4", int64(totalSize), firstURL)
	require.NoError(t, err)
	path := upload.BasePath + created.UploadID
	client := &http.Client{
		Transport:     &http.Transport{ExpectContinueTimeout: time.Second},
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()

	type reply struct {
		status int
		header http.Header
		err    error
	}
	send := func(reqCtx context.Context, method, base string, body io.Reader, size, offset int, expectContinue bool) reply {
		req, err := http.NewRequestWithContext(reqCtx, method, base+path, body)
		if err != nil {
			return reply{err: err}
		}
		req.Header.Set("Authorization", "Bearer "+created.UploadToken)
		req.Header.Set("Tus-Resumable", "1.0.0")
		if method == http.MethodPatch {
			req.ContentLength = int64(size)
			req.Header.Set("Content-Type", "application/offset+octet-stream")
			req.Header.Set("Upload-Offset", strconv.Itoa(offset))
			if expectContinue {
				req.Header.Set("Expect", "100-continue")
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			return reply{err: err}
		}
		_, readErr := io.Copy(io.Discard, resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return reply{err: readErr}
		}
		return reply{status: resp.StatusCode, header: resp.Header, err: closeErr}
	}

	// Hold the first PATCH mid-body until the competing PATCH is on the wire.
	raceCtx, stopRace := context.WithTimeout(ctx, 15*time.Second)
	defer stopRace()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	firstReading, secondSent := make(chan struct{}), make(chan struct{})
	firstCtx := httptrace.WithClientTrace(raceCtx, &httptrace.ClientTrace{Got100Continue: sync.OnceFunc(func() { close(firstReading) })})
	secondCtx := httptrace.WithClientTrace(raceCtx, &httptrace.ClientTrace{WroteHeaders: sync.OnceFunc(func() { close(secondSent) })})
	firstReply, secondReply := make(chan reply, 1), make(chan reply, 1)
	go func() { firstReply <- send(firstCtx, http.MethodPatch, firstURL, reader, prefixSize, 0, true) }()
	select {
	case <-firstReading:
	case r := <-firstReply:
		t.Fatalf("first PATCH returned before reading its body: status %d, error %v", r.status, r.err)
	case <-raceCtx.Done():
		t.Fatal("first PATCH never started reading its body")
	}
	_, err = io.WriteString(writer, prefixes[0][:1])
	require.NoError(t, err)
	go func() {
		secondReply <- send(secondCtx, http.MethodPatch, secondURL, strings.NewReader(prefixes[1]), prefixSize, 0, false)
	}()
	select {
	case <-secondSent:
	case <-raceCtx.Done():
		t.Fatal("competing PATCH never sent its headers")
	}
	_, _ = io.WriteString(writer, prefixes[0][1:]) // A preempted request may close the pipe.
	require.NoError(t, writer.Close())
	responses := [2]reply{<-firstReply, <-secondReply}
	for _, r := range responses {
		require.NoError(t, r.err)
	}
	winner, loser := 0, 1
	if responses[1].status == http.StatusNoContent {
		winner, loser = 1, 0
	}
	require.Equal(t, http.StatusNoContent, responses[winner].status)
	stale := responses[loser]
	if stale.status == http.StatusServiceUnavailable {
		loserURL := [2]string{firstURL, secondURL}[loser]
		stale = send(ctx, http.MethodPatch, loserURL, strings.NewReader(prefixes[loser]), prefixSize, 0, false)
		require.NoError(t, stale.err)
	}
	require.Equal(t, http.StatusConflict, stale.status, "only one PATCH may commit at offset zero")
	closeFirst()
	closeSecond()

	state, _, coldURL, _ := openWorker()
	checkHead := func(offset int) {
		r := send(ctx, http.MethodHead, coldURL, nil, 0, 0, false)
		require.NoError(t, r.err)
		require.Equal(t, http.StatusOK, r.status)
		require.Equal(t, strconv.Itoa(offset), r.header.Get("Upload-Offset"))
		require.Equal(t, strconv.Itoa(totalSize), r.header.Get("Upload-Length"))
	}
	checkHead(prefixSize)
	r := send(ctx, http.MethodPatch, coldURL, strings.NewReader(tail), len(tail), prefixSize, false)
	require.NoError(t, r.err)
	require.Equal(t, http.StatusNoContent, r.status)

	taskKey := "vod-process:" + created.UploadID
	task, err := state.GetTaskByKey(ctx, taskKey)
	require.NoError(t, err)
	require.NotNil(t, task)
	row, err := state.GetUpload(ctx, created.UploadID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.NotNil(t, row.CompletedAt)
	checkHead(totalSize)
	store := blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)
	objectKey, ok := store.ParseLocation(row.Location)
	require.True(t, ok, "invalid upload location: %q", row.Location)
	raw, err := store.Open(ctx, objectKey)
	require.NoError(t, err)
	defer raw.Close()
	require.Equal(t, int64(totalSize), raw.Size())
	got, err := io.ReadAll(io.NewSectionReader(raw, 0, raw.Size()))
	require.NoError(t, err)
	require.Len(t, got, totalSize)
	require.True(t, bytes.Equal(got[prefixSize:], []byte(tail)), "raw object has the wrong tail")
	// Delete only the object proven to belong to this fixture.
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		require.NoError(t, store.Delete(cleanupCtx, objectKey))
	})
	require.True(t, bytes.Equal(got[:prefixSize], []byte(prefixes[winner])), "raw object has the wrong winning prefix")

	const worker = "station-worker"
	claim, err := state.DequeueTask(ctx, worker, statedb.TaskVODProcess)
	require.NoError(t, err)
	require.NotNil(t, claim)
	require.NotEmpty(t, claim.ClaimToken)
	require.Equal(t, &taskKey, claim.TaskKey)

	recovery, _, _ := openState()
	expired := recovery.DB.WithContext(ctx).Model(&statedb.AppTask{}).Where("id = ?", claim.ID).
		UpdateColumn("lock_expires", time.Now().UTC().Add(-time.Minute))
	require.NoError(t, expired.Error)
	require.EqualValues(t, 1, expired.RowsAffected)
	newClaim, err := recovery.DequeueTask(ctx, worker, statedb.TaskVODProcess)
	require.NoError(t, err)
	require.NotNil(t, newClaim)
	require.Equal(t, claim.ID, newClaim.ID)
	require.NotEmpty(t, newClaim.ClaimToken)
	require.NotEqual(t, claim.ClaimToken, newClaim.ClaimToken)
	require.Error(t, state.CompleteTask(ctx, claim.ID, claim.ClaimToken))
	require.NoError(t, recovery.CompleteTask(ctx, newClaim.ID, newClaim.ClaimToken))

	var tasks []statedb.AppTask
	require.NoError(t, recovery.DB.WithContext(ctx).Where("type = ?", statedb.TaskVODProcess).Find(&tasks).Error)
	require.Len(t, tasks, 1)
	require.Equal(t, claim.ID, tasks[0].ID)
	require.Equal(t, statedb.TaskStatusCompleted, tasks[0].Status)
}
