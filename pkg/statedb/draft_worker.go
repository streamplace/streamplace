package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

// Take the owner lock before entering the claim transaction. Do not hold a
// SQL transaction across a user-space API call, or a writer lock while
// waiting for an API call that already owns this lock.
func (state *StatefulDB) withDraftTaskWrite(ctx context.Context, did string, write func(*StatefulDB) error) error {
	unlock, err := state.lockDrafts(ctx, did)
	if err != nil {
		return err
	}
	defer unlock()
	return state.withTaskWrite(ctx, write)
}

// CreateUploadDraftTx anchors a worker draft in the transaction that owns
// the upload. Callers serialize on the owner lock or an exclusive SQLite
// station write transaction. A nonempty upload link survives deletion and
// reassignment: redelivering completion must not manufacture a new draft.
func (state *StatefulDB) CreateUploadDraftTx(ctx context.Context, tx *gorm.DB, did, uploadID string, rec *placestream.VodDraftVideo) (*DraftVideo, error) {
	db := &StatefulDB{DB: tx}
	upload, err := db.GetUpload(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.RepoDID != did {
		return nil, errors.New("draft worker upload not found")
	}
	if upload.DraftURI != "" {
		return nil, nil
	}
	if existing, err := db.getDraftByUpload(ctx, uploadID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, tx.WithContext(ctx).Model(upload).Update("draft_uri", existing.URI).Error
	}
	if rec == nil {
		rec = &placestream.VodDraftVideo{Title: uploadDraftTitle(upload.Filename), Status: "processing", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	operational := *rec
	if err := applyUploadResult(upload, &operational); err != nil {
		return nil, err
	}
	cutover, err := db.GetConfig(draftCutoverKey(did))
	if err != nil {
		return nil, err
	}
	mode := ""
	if cutover != nil {
		mode = draftPending
		setDraftMetadata(&operational, &placestream.VodDraftMetadata{})
	}
	return db.insertDraft(ctx, did, uploadID, mode, upload.ContentCID, &operational)
}

func applyUploadResult(upload *Upload, rec *placestream.VodDraftVideo) error {
	switch upload.ProcessingStatus {
	case "done":
		source, err := draftSourceFromTrackURIs(upload.TrackURIs)
		if err != nil {
			return err
		}
		rec.Status, rec.Source, rec.DurationMs, rec.Error = "ready", source, &upload.DurationMS, nil
	case "error":
		rec.Status, rec.Error = "error", &upload.ProcessingError
	}
	return nil
}

// syncDraftResult is called under the same DID lock as migrations and edits,
// inside the task transaction. It never reads or writes the private space.
func (state *StatefulDB) syncDraftResult(ctx context.Context, uploadID string) error {
	row, err := state.getDraftByUpload(ctx, uploadID)
	if err != nil || row == nil {
		return err
	}
	upload, err := state.GetUpload(ctx, uploadID)
	if err != nil || upload == nil {
		return err
	}
	rec, err := row.Record()
	if err != nil {
		return err
	}
	if err := applyUploadResult(upload, rec); err != nil {
		return err
	}
	data, cid, err := marshalDraft(rec)
	if err != nil {
		return err
	}
	return state.DB.WithContext(ctx).Model(row).Updates(map[string]any{"data": data, "cid": cid, "content_cid": upload.ContentCID}).Error
}

func uploadDraftTitle(filename string) string {
	if filename != "" {
		return filename
	}
	return "Uploaded video"
}

// A fallback draft is created only on the last, pre-dispatch publication
// failure. Its terminal job no longer needs the proposed public record once
// the draft moves; do not keep those editable fields in the queue either.
type fallbackDraftSeed struct {
	taskID  uint
	publish *VideoDraft
	payload json.RawMessage
}

var errDraftJobActive = errors.New("draft publication job is not terminal or has outstanding effects")

const noPublicationEffects = "NOT EXISTS (SELECT 1 FROM app_task_effects WHERE task_id = app_tasks.id)"

func (state *StatefulDB) fallbackDraftSeed(ctx context.Context, dv *DraftVideo) (*fallbackDraftSeed, error) {
	if dv.OriginUploadID == "" {
		return nil, nil
	}
	var row struct {
		ID        uint
		Status    TaskStatus
		Payload   json.RawMessage
		NoEffects bool
	}
	result := state.DB.WithContext(ctx).Model(&AppTask{}).
		Select("id, status, payload, "+noPublicationEffects+" AS no_effects").
		Where("task_key = ? AND type = ?", "finalize-vod:"+dv.OriginUploadID, TaskFinalizeLivestreamVOD).Find(&row)
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	var job FinalizeLivestreamVODTask
	if err := json.Unmarshal(row.Payload, &job); err != nil {
		return nil, err
	}
	if job.Publish == nil || job.UploadID != dv.OriginUploadID || job.RepoDID != dv.UserDID {
		return nil, nil
	}
	if !row.NoEffects || (row.Status != TaskStatusCompleted && row.Status != TaskStatusFailed) {
		return nil, errDraftJobActive
	}
	seed := &fallbackDraftSeed{taskID: row.ID, publish: job.Publish}
	job.Publish = nil
	payload, err := json.Marshal(&job)
	if err != nil {
		return nil, err
	}
	seed.payload = payload
	return seed, nil
}

func (state *StatefulDB) pendingDraftMetadata(ctx context.Context, dv *DraftVideo, rec *placestream.VodDraftVideo, seed *fallbackDraftSeed) (placestream.VodDraftMetadata, error) {
	if seed != nil {
		return draftMetadata(LivestreamDraftRecord(seed.publish)), nil
	}
	upload, err := state.GetUpload(ctx, dv.OriginUploadID)
	if err != nil {
		return placestream.VodDraftMetadata{}, err
	}
	if upload == nil || upload.RepoDID != dv.UserDID {
		return placestream.VodDraftMetadata{Title: "Untitled"}, nil
	}
	if upload.Backend != livestreamUploadBackend {
		return placestream.VodDraftMetadata{Title: uploadDraftTitle(upload.Filename)}, nil
	}
	// Older jobs may no longer exist. Rebuild defaults from public input.
	if len(rec.Connections) != 0 {
		ls, err := state.model.GetLivestream(upload.Location)
		if err != nil {
			return placestream.VodDraftMetadata{}, err
		}
		if ls != nil && ls.RepoDID == dv.UserDID {
			view, err := ls.ToLivestreamView()
			if err != nil {
				return placestream.VodDraftMetadata{}, err
			}
			if live, ok := view.Record.Val.(*placestream.Livestream); ok {
				video := VideoDraftForLivestreams([]LivestreamItem{{Livestream: ls, Record: live}}, "", "")
				return draftMetadata(LivestreamDraftRecord(video)), nil
			}
		}
	}
	return placestream.VodDraftMetadata{Title: "Livestream"}, nil
}
