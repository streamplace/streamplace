package statedb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestUserPreferences(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		prefs, err := state.GetUserPreferences(ctx, "did:plc:prefs")
		require.NoError(t, err)
		require.False(t, prefs.AutoPublishVODs, "off until the user opts in")

		prefs.AutoPublishVODs = true
		require.NoError(t, state.PutUserPreferences(ctx, prefs))
		prefs, err = state.GetUserPreferences(ctx, "did:plc:prefs")
		require.NoError(t, err)
		require.True(t, prefs.AutoPublishVODs)

		prefs.AutoPublishVODs = false
		require.NoError(t, state.PutUserPreferences(ctx, prefs), "saving over existing preferences")
		prefs, err = state.GetUserPreferences(ctx, "did:plc:prefs")
		require.NoError(t, err)
		require.False(t, prefs.AutoPublishVODs)

		other, err := state.GetUserPreferences(ctx, "did:plc:other")
		require.NoError(t, err)
		require.False(t, other.AutoPublishVODs, "one user's preferences are not another's")
	})
}

func setAutoPublishVODs(t *testing.T, state *StatefulDB, did string, on bool) {
	t.Helper()
	require.NoError(t, state.PutUserPreferences(context.Background(), &UserPreferences{RepoDID: did, AutoPublishVODs: on}))
}

// endedLivestream seeds an ended livestream record for did, a streamer opted
// in to automatic VOD publishing.
func endedLivestream(t *testing.T, state *StatefulDB, did string) string {
	t.Helper()
	setAutoPublishVODs(t, state, did, true)
	return seedLivestream(t, state.model, did, strings.TrimPrefix(did, "did:plc:"), time.Hour, &placestream.Livestream{
		LexiconTypeID: "place.stream.livestream",
		CreatedAt:     time.Now().Add(-time.Hour).Format(time.RFC3339),
		EndedAt:       ptr(time.Now().Format(time.RFC3339)),
		Title:         "Speedrun night",
	})
}

// recordObject records one live-recording object for the livestream,
// completed or still being uploaded.
func recordObject(t *testing.T, state *StatefulDB, did, uri, key string, completed bool) {
	t.Helper()
	ctx := context.Background()
	id, err := state.RecordStart(ctx, did, "bucket", key, uri, time.Now())
	require.NoError(t, err)
	if completed {
		require.NoError(t, state.RecordComplete(ctx, id, 1, 100))
	}
}

func pendingTasks(t *testing.T, state *StatefulDB, taskType string) []AppTask {
	t.Helper()
	tasks, err := state.ListTasks(context.Background(), TaskFilters{Type: taskType, Status: TaskStatusPending, Limit: 10})
	require.NoError(t, err)
	return tasks
}

// runAutoPublish schedules the decision for the livestream and runs the
// task it queued.
func runAutoPublish(t *testing.T, state *StatefulDB, uri string) {
	t.Helper()
	require.NoError(t, state.ScheduleAutoPublishVOD(context.Background(), uri))
	tasks := pendingTasks(t, state, TaskAutoPublishVOD)
	require.Len(t, tasks, 1)
	require.NoError(t, state.processAutoPublishVODTask(context.Background(), &tasks[0]))
}

func TestScheduleAutoPublishVOD(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		uri := endedLivestream(t, state, "did:plc:scheduled")
		before := time.Now()
		require.NoError(t, state.ScheduleAutoPublishVOD(ctx, uri))
		require.NoError(t, state.ScheduleAutoPublishVOD(ctx, uri), "the ended record seen again")
		tasks := pendingTasks(t, state, TaskAutoPublishVOD)
		require.Len(t, tasks, 1, "one decision per livestream")
		require.NotNil(t, tasks[0].ScheduledAt)
		require.False(t, tasks[0].ScheduledAt.Before(before.Add(autoPublishVODWait)), "gives the recorder time to complete its last object")
	})
}

func TestAutoPublishVODQueuesPublishingFinalize(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		did := "did:plc:autovod"
		uri := endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		recordObject(t, state, did, uri, "b.m4s", true)

		runAutoPublish(t, state, uri)

		require.Empty(t, pendingTasks(t, state, TaskAutoPublishVOD))
		vods := pendingTasks(t, state, TaskFinalizeLivestreamVOD)
		require.Len(t, vods, 1)
		var vt FinalizeLivestreamVODTask
		require.NoError(t, json.Unmarshal(vods[0].Payload, &vt))
		require.Equal(t, did, vt.RepoDID)
		require.Equal(t, uri, vt.LivestreamURI)
		require.NotNil(t, vt.Publish, "published, not left as a draft")
		require.Equal(t, "Speedrun night", vt.Publish.Title)
		require.Len(t, vt.Publish.Connections, 1)
		require.Equal(t, uri, vt.Publish.Connections[0].Video_Connection.Ref.Uri)

		upload, err := state.GetUpload(context.Background(), vt.UploadID)
		require.NoError(t, err)
		require.Equal(t, did, upload.RepoDID)
		require.Equal(t, uri, upload.Location)
	})
}

// A livestream that ended while the streamer had it off stays unpublished
// when they turn it on and the node sees the record again.
func TestAutoPublishVODDecidesOnce(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		did := "did:plc:decided"
		uri := endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		setAutoPublishVODs(t, state, did, false)
		runAutoPublish(t, state, uri)
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))

		setAutoPublishVODs(t, state, did, true)
		require.NoError(t, state.ScheduleAutoPublishVOD(context.Background(), uri))
		require.Empty(t, pendingTasks(t, state, TaskAutoPublishVOD))
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))
	})
}

// Sync queues the task before it indexes the ended record; the task waits
// for the index to show the end.
func TestAutoPublishVODWaitsForEndedRecord(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:notindexed"
		uri := "at://" + did + "/place.stream.livestream/notindexed"
		runAutoPublish(t, state, uri)
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))
		tasks := pendingTasks(t, state, TaskAutoPublishVOD)
		require.Len(t, tasks, 1, "waits another pass")

		require.Equal(t, uri, endedLivestream(t, state, did))
		recordObject(t, state, did, uri, "a.m4s", true)
		require.NoError(t, state.processAutoPublishVODTask(ctx, &tasks[0]))
		require.Len(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD), 1)
	})
}

// A pass that dies after handing off (its task is never completed) runs
// again; it must pick up the VOD it started, not start a second one.
func TestAutoPublishVODRetriedHandoff(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:interrupted"
		uri := endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		require.NoError(t, state.ScheduleAutoPublishVOD(ctx, uri))
		tasks := pendingTasks(t, state, TaskAutoPublishVOD)
		require.Len(t, tasks, 1)

		require.NoError(t, state.processAutoPublishVODTask(ctx, &tasks[0]))
		require.NoError(t, state.processAutoPublishVODTask(ctx, &tasks[0]))

		require.Len(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD), 1, "one VOD")
		uploads, err := state.ListLivestreamUploads(ctx, uri)
		require.NoError(t, err)
		require.Len(t, uploads, 1, "one upload")
	})
}

func TestAutoPublishVODWaitsForRecording(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		did := "did:plc:stillwriting"
		uri := endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		recordObject(t, state, did, uri, "b.m4s", false)

		runAutoPublish(t, state, uri)

		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD), "not before the last object completes")
		tasks := pendingTasks(t, state, TaskAutoPublishVOD)
		require.Len(t, tasks, 1, "waits another pass")
		var next AutoPublishVODTask
		require.NoError(t, json.Unmarshal(tasks[0].Payload, &next))
		require.Equal(t, 1, next.Waits)
	})
}

func TestAutoPublishVODStopsWaitingForAbandonedObject(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()
		did := "did:plc:abandoned"
		uri := endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		recordObject(t, state, did, uri, "b.m4s", false)

		task, err := state.EnqueueTask(ctx, TaskAutoPublishVOD, AutoPublishVODTask{LivestreamURI: uri, Waits: autoPublishVODMaxWaits})
		require.NoError(t, err)
		require.NoError(t, state.processAutoPublishVODTask(ctx, task))

		require.Len(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD), 1, "the VOD of the objects that completed")
	})
}

func TestAutoPublishVODSkips(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := context.Background()

		// No recording: the streamer was not recorded (not in the VOD beta,
		// recording turned off, no S3 on this node).
		uri := endedLivestream(t, state, "did:plc:unrecorded")
		runAutoPublish(t, state, uri)
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))

		// Turned off between the livestream ending and the task running.
		did := "did:plc:changedmind"
		uri = endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		require.NoError(t, state.ScheduleAutoPublishVOD(ctx, uri))
		setAutoPublishVODs(t, state, did, false)
		tasks := pendingTasks(t, state, TaskAutoPublishVOD)
		require.Len(t, tasks, 1)
		require.NoError(t, state.processAutoPublishVODTask(ctx, &tasks[0]))
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))
		require.Empty(t, pendingTasks(t, state, TaskAutoPublishVOD))

		// Finalized by hand (the Finalize button, or a moderator) before
		// the task ran.
		did = "did:plc:byhand"
		uri = endedLivestream(t, state, did)
		recordObject(t, state, did, uri, "a.m4s", true)
		require.NoError(t, state.CreateLivestreamUpload(ctx, "by-hand", did, uri))
		runAutoPublish(t, state, uri)
		require.Empty(t, pendingTasks(t, state, TaskFinalizeLivestreamVOD))
	})
}
