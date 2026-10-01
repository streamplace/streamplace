package statedb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

// AutoPublishVODTask decides, once per livestream, whether to publish the VOD
// of a livestream the node has just seen end: it does if the streamer has
// UserPreferences.AutoPublishVODs on when the task runs. It is queued by
// ScheduleAutoPublishVOD when sync first sees the livestream record ended.
type AutoPublishVODTask struct {
	LivestreamURI string `json:"livestreamURI"`
	// Waits counts the passes that found the ended record not indexed yet,
	// or part of the recording still being written.
	Waits int `json:"waits,omitempty"`
}

const (
	// autoPublishVODWait is how long the task waits after the livestream ends,
	// and between passes that have to wait. The recorder completes its last
	// object when the next segment arrives or the stream session ends, a few
	// seconds after the record ends.
	autoPublishVODWait = 15 * time.Second
	// autoPublishVODMaxWaits bounds that waiting: an object whose upload never
	// completes (its node died mid-stream) would otherwise hold the VOD back
	// forever, so after this many passes the VOD is made of the objects that
	// did complete.
	autoPublishVODMaxWaits = 8
)

// ScheduleAutoPublishVOD queues the decision on publishing the VOD of a
// livestream the node has just seen end, whatever the streamer's preference:
// the task reads it when it runs. Once per livestream, so a livestream that
// ended while the streamer had it off stays decided when they turn it on.
func (state *StatefulDB) ScheduleAutoPublishVOD(ctx context.Context, livestreamURI string) error {
	return state.enqueueAutoPublishVOD(ctx, AutoPublishVODTask{LivestreamURI: livestreamURI})
}

func (state *StatefulDB) enqueueAutoPublishVOD(ctx context.Context, t AutoPublishVODTask) error {
	key := fmt.Sprintf("auto-publish-vod::%s::%d", t.LivestreamURI, t.Waits)
	_, err := state.EnqueueTask(ctx, TaskAutoPublishVOD, t, WithTaskKey(key), WithScheduledAt(time.Now().Add(autoPublishVODWait).UTC()))
	return err
}

func (state *StatefulDB) processAutoPublishVODTask(ctx context.Context, task *AppTask) error {
	ctx = log.WithLogValues(ctx, "func", "processAutoPublishVODTask")
	var t AutoPublishVODTask
	if err := json.Unmarshal(task.Payload, &t); err != nil {
		return err
	}
	ctx = log.WithLogValues(ctx, "livestream", t.LivestreamURI)
	again := func(why string) error {
		next := t
		next.Waits++
		if err := state.enqueueAutoPublishVOD(ctx, next); err != nil {
			return fmt.Errorf("reschedule automatic VOD publishing: %w", err)
		}
		log.Debug(ctx, why, "waits", next.Waits)
		return state.CompleteTask(ctx, task.ID)
	}

	// Sync queues this before it indexes the ended record, so the record
	// may not show the end yet.
	ls, err := state.model.GetLivestream(t.LivestreamURI)
	if err != nil {
		return fmt.Errorf("get livestream: %w", err)
	}
	var rec *placestream.Livestream
	if ls != nil {
		view, err := ls.ToLivestreamView()
		if err != nil {
			return fmt.Errorf("decode livestream: %w", err)
		}
		var ok bool
		if rec, ok = view.Record.Val.(*placestream.Livestream); !ok {
			return fmt.Errorf("record is not a place.stream.livestream: %s", ls.URI)
		}
	}
	if rec == nil || rec.EndedAt == nil {
		if t.Waits < autoPublishVODMaxWaits {
			return again("ended livestream not indexed yet; waiting to decide on its VOD")
		}
		log.Warn(ctx, "ended livestream was never indexed; no VOD to publish")
		return state.CompleteTask(ctx, task.ID)
	}
	ctx = log.WithLogValues(ctx, "did", ls.RepoDID)

	prefs, err := state.GetUserPreferences(ctx, ls.RepoDID)
	if err != nil {
		return fmt.Errorf("get user preferences: %w", err)
	}
	if !prefs.AutoPublishVODs {
		log.Debug(ctx, "automatic VOD publishing is off; not publishing")
		return state.CompleteTask(ctx, task.ID)
	}

	// The upload is named after the livestream, so a pass interrupted after
	// creating it (or after queueing the finalize) picks it up again rather
	// than starting a second VOD. Any other upload of the livestream is one
	// the streamer (or a moderator) finalized by hand.
	uploadID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(ls.URI)).String()
	uploads, err := state.ListLivestreamUploads(ctx, ls.URI)
	if err != nil {
		return fmt.Errorf("list livestream uploads: %w", err)
	}
	created := false
	for _, u := range uploads {
		if u.ID != uploadID {
			log.Log(ctx, "livestream was already finalized into a VOD; not publishing another", "uploadId", u.ID)
			return state.CompleteTask(ctx, task.ID)
		}
		created = true
	}

	open, err := state.CountOpenS3Segments(ctx, ls.URI)
	if err != nil {
		return fmt.Errorf("count open recording objects: %w", err)
	}
	if open > 0 && t.Waits < autoPublishVODMaxWaits {
		return again("livestream recording still being written; waiting to publish its VOD")
	}
	segs, err := state.ListS3SegmentsForLivestream(ctx, ls.URI)
	if err != nil {
		return fmt.Errorf("list recorded objects: %w", err)
	}
	if len(segs) == 0 {
		log.Log(ctx, "livestream has no recording; no VOD to publish")
		return state.CompleteTask(ctx, task.ID)
	}

	if !created {
		if err := state.CreateLivestreamUpload(ctx, uploadID, ls.RepoDID, ls.URI); err != nil {
			return fmt.Errorf("create upload: %w", err)
		}
	}
	vodTask := FinalizeLivestreamVODTask{
		UploadID:       uploadID,
		RepoDID:        ls.RepoDID,
		LivestreamURI:  ls.URI,
		LivestreamURIs: []string{ls.URI},
		Publish:        VideoDraftForLivestreams([]LivestreamItem{{Livestream: ls, Record: rec}}, "", ""),
	}
	if _, err := state.EnqueueTask(ctx, TaskFinalizeLivestreamVOD, vodTask, WithTaskKey("finalize-vod:"+uploadID)); err != nil {
		return fmt.Errorf("enqueue VOD finalize: %w", err)
	}
	log.Log(ctx, "livestream ended; VOD queued for automatic publishing", "uploadId", uploadID, "objects", len(segs), "waits", t.Waits)
	return state.CompleteTask(ctx, task.ID)
}
