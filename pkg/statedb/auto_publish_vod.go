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

// AutoPublishVODTask publishes the VOD of a livestream the node has just seen
// end, for a streamer who had UserPreferences.AutoPublishVODs on then and
// still has it on when the task runs. It is queued by ScheduleAutoPublishVOD.
type AutoPublishVODTask struct {
	LivestreamURI string `json:"livestreamURI"`
	// Waits counts the passes that found part of the recording still being
	// written.
	Waits int `json:"waits,omitempty"`
}

const (
	// autoPublishVODWait is how long the task waits after the livestream ends,
	// and between passes that find the recording still being written. The
	// recorder completes its last object when the next segment arrives or the
	// stream session ends, a few seconds after the record ends.
	autoPublishVODWait = 15 * time.Second
	// autoPublishVODMaxWaits bounds that waiting: an object whose upload never
	// completes (its node died mid-stream) would otherwise hold the VOD back
	// forever, so after this many passes the VOD is made of the objects that
	// did complete.
	autoPublishVODMaxWaits = 8
)

// ScheduleAutoPublishVOD queues the publishing of the VOD of a livestream the
// node has just seen end, if its streamer has automatic VOD publishing on.
// Once per livestream: scheduling it again is a no-op.
func (state *StatefulDB) ScheduleAutoPublishVOD(ctx context.Context, repoDID, livestreamURI string) error {
	prefs, err := state.GetUserPreferences(ctx, repoDID)
	if err != nil {
		return fmt.Errorf("get user preferences: %w", err)
	}
	if !prefs.AutoPublishVODs {
		return nil
	}
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
	ls, err := state.model.GetLivestream(t.LivestreamURI)
	if err != nil {
		return fmt.Errorf("get livestream: %w", err)
	}
	if ls == nil {
		log.Warn(ctx, "livestream to publish a VOD of is not indexed; skipping")
		return state.CompleteTask(ctx, task.ID)
	}
	ctx = log.WithLogValues(ctx, "did", ls.RepoDID)
	view, err := ls.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("decode livestream: %w", err)
	}
	rec, ok := view.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("record is not a place.stream.livestream: %s", ls.URI)
	}

	// The streamer may have turned it off since the livestream ended.
	prefs, err := state.GetUserPreferences(ctx, ls.RepoDID)
	if err != nil {
		return fmt.Errorf("get user preferences: %w", err)
	}
	if !prefs.AutoPublishVODs {
		log.Log(ctx, "automatic VOD publishing was turned off after the livestream ended; skipping")
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
		next := t
		next.Waits++
		if err := state.enqueueAutoPublishVOD(ctx, next); err != nil {
			return fmt.Errorf("reschedule automatic VOD publishing: %w", err)
		}
		log.Debug(ctx, "livestream recording still being written; waiting to publish its VOD", "open", open, "waits", next.Waits)
		return state.CompleteTask(ctx, task.ID)
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
