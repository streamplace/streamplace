package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
)

var (
	ErrDraftNotFound    = errors.New("draft not found")
	ErrDraftReauthorize = errors.New("reauthorize to access private drafts")
	ErrDraftCursor      = errors.New("invalid draft cursor")
)

const (
	draftPending         = "pending"
	draftInSpace         = "space"
	cutoverStarted  byte = 1
	cutoverComplete byte = 2
)

func draftCutoverKey(did string) string {
	return "draft-cutover:" + spaces.DraftMetadataCollection + ":" + did
}

func (state *StatefulDB) lockDrafts(ctx context.Context, did string) (func(), error) {
	return state.WaitForNamedLock(ctx, draftCutoverKey(did))
}

// Drafts holds the per-owner lock across a draft operation, including publish.
// A non-nil client must be the caller's client with the expanded draft grant.
type Drafts struct {
	state        *StatefulDB
	did          string
	space        spaces.Client
	unlock       func()
	migratedOnly bool
}

func (state *StatefulDB) OpenDrafts(ctx context.Context, did string, client spaces.Client) (*Drafts, error) {
	return state.openDrafts(ctx, did, client, false)
}

// OpenMigratedDrafts binds an action-limited PDS client to an already
// migrated collection. This handle cannot initiate migration, reconcile
// pending worker records, or expose a legacy row.
func (state *StatefulDB) OpenMigratedDrafts(ctx context.Context, did string, client spaces.Client) (*Drafts, error) {
	if client == nil {
		return nil, ErrDraftReauthorize
	}
	return state.openDrafts(ctx, did, client, true)
}

func (state *StatefulDB) openDrafts(ctx context.Context, did string, client spaces.Client, migratedOnly bool) (*Drafts, error) {
	unlock, err := state.lockDrafts(ctx, did)
	if err != nil {
		return nil, err
	}
	d := &Drafts{state: state, did: did, space: client, unlock: unlock, migratedOnly: migratedOnly}
	if err := d.prepare(ctx); err != nil {
		unlock()
		return nil, err
	}
	return d, nil
}

func (d *Drafts) Close() { d.unlock() }

func (d *Drafts) prepare(ctx context.Context) error {
	key := draftCutoverKey(d.did)
	cutover, err := d.state.GetConfig(key)
	if err != nil {
		return err
	}
	complete := cutover != nil && len(cutover.Value) == 1 && cutover.Value[0] == cutoverComplete
	if d.migratedOnly {
		if !complete {
			return ErrDraftReauthorize
		}
		return nil
	}
	if d.space == nil {
		if cutover != nil {
			return ErrDraftReauthorize
		}
		return nil
	}
	// Persist the decision before any remote writes. Partial migrations must
	// also reject sessions without the grant, even when no draft rows remain.
	if cutover == nil {
		if err := d.state.PutConfig(key, []byte{cutoverStarted}); err != nil {
			return err
		}
	}
	var rows []DraftVideo
	if err := d.state.DB.WithContext(ctx).Where("user_did = ? AND (metadata_state IS NULL OR metadata_state <> ?)", d.did, draftInSpace).
		Order("created_at, uri").Find(&rows).Error; err != nil {
		return err
	}
	activeJob := false
	for i := range rows {
		row := &rows[i]
		rec, err := row.Record()
		if err != nil {
			return err
		}
		seed, err := d.state.fallbackDraftSeed(ctx, row)
		if errors.Is(err, errDraftJobActive) {
			// The worker created its fallback before acknowledging the job.
			// Let other drafts proceed; this one is not visible yet.
			activeJob = true
			continue
		}
		if err != nil {
			return err
		}
		metadata := draftMetadata(rec)
		if row.MetadataState == draftPending {
			metadata, err = d.state.pendingDraftMetadata(ctx, row, rec, seed)
			if err != nil {
				return err
			}
		}
		if err := d.materialize(ctx, row, rec, &metadata, seed); err != nil {
			return err
		}
	}
	if !complete && !activeJob {
		if len(rows) == 0 {
			// A later create-only grant cannot provision a missing space.
			if err := spaces.EnsureDraftSpace(ctx, d.space, d.did); err != nil {
				return fmt.Errorf("ensure personal space: %w", err)
			}
		}
		return d.state.PutConfig(key, []byte{cutoverComplete})
	}
	return nil
}

// materialize checkpoints each row separately; a later failing row does not
// cause a retry to recreate one already moved. Workers use the same DID lock.
func (d *Drafts) materialize(ctx context.Context, dv *DraftVideo, rec *placestream.VodDraftVideo, metadata *placestream.VodDraftMetadata, seed *fallbackDraftSeed) error {
	setDraftMetadata(rec, &placestream.VodDraftMetadata{})
	data, cid, err := marshalDraft(rec)
	if err != nil {
		return err
	}
	rkey := draftRkey(dv.URI)
	observed := false
	if dv.MetadataAttemptRev != nil {
		observed, err = spaces.DraftCreateObserved(ctx, d.space, d.did, rkey, *dv.MetadataAttemptRev)
		if err != nil {
			return fmt.Errorf("reconcile draft metadata: %w", err)
		}
	} else {
		rev, err := spaces.LatestRev(ctx, d.space, d.did)
		if err != nil {
			return err
		}
		if rev == "" {
			if err := spaces.EnsureDraftSpace(ctx, d.space, d.did); err != nil {
				return fmt.Errorf("ensure personal space: %w", err)
			}
		}
		if err := d.state.DB.WithContext(ctx).Model(dv).Update("metadata_attempt_rev", rev).Error; err != nil {
			return err
		}
	}
	if !observed {
		// RecordAlreadyExists is success for this move: the PDS value wins.
		if _, err := spaces.CreateDraftMetadata(ctx, d.space, d.did, rkey, metadata); err != nil {
			return fmt.Errorf("materialize draft metadata: %w", err)
		}
	}
	updates := map[string]any{"data": data, "cid": cid, "metadata_state": draftInSpace, "metadata_attempt_rev": nil}
	db := d.state.DB.WithContext(ctx)
	if seed == nil {
		return db.Model(dv).Updates(updates).Error
	}
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&AppTask{}).
			Where("id = ? AND status IN (?,?) AND "+noPublicationEffects, seed.taskID, TaskStatusCompleted, TaskStatusFailed).
			UpdateColumn("payload", seed.payload)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errDraftJobActive
		}
		return tx.Model(dv).Updates(updates).Error
	})
}

func draftRkey(uri string) string { return uri[strings.LastIndexByte(uri, '/')+1:] }

func draftMetadata(rec *placestream.VodDraftVideo) placestream.VodDraftMetadata {
	return placestream.VodDraftMetadata{
		Title: rec.Title, Description: rec.Description, DescriptionFacets: rec.DescriptionFacets,
		Thumb: rec.Thumb, Tags: rec.Tags, Activity: (*placestream.VodDraftMetadata_Activity)(rec.Activity),
		ContentWarnings: rec.ContentWarnings, ContentRights: rec.ContentRights,
	}
}

func setDraftMetadata(rec *placestream.VodDraftVideo, metadata *placestream.VodDraftMetadata) {
	rec.Title, rec.Description, rec.DescriptionFacets = metadata.Title, metadata.Description, metadata.DescriptionFacets
	rec.Thumb, rec.Tags, rec.Activity = metadata.Thumb, metadata.Tags, (*placestream.VodDraftVideo_Activity)(metadata.Activity)
	rec.ContentWarnings, rec.ContentRights = metadata.ContentWarnings, metadata.ContentRights
}

func joinDraftMetadata(dv *DraftVideo, raw *spaces.Record) error {
	var metadata placestream.VodDraftMetadata
	if err := json.Unmarshal(raw.Value, &metadata); err != nil || metadata.LexiconTypeID != spaces.DraftMetadataCollection {
		return errors.New("invalid private draft metadata")
	}
	rec, err := dv.Record()
	if err != nil {
		return err
	}
	setDraftMetadata(rec, &metadata)
	dv.Data, dv.CID, err = marshalDraft(rec)
	return err
}

func (d *Drafts) Create(ctx context.Context, uploadID string, rec *placestream.VodDraftVideo) (*DraftVideo, error) {
	if d.space == nil {
		return d.state.insertDraft(ctx, d.did, uploadID, "", "", rec)
	}
	if !d.migratedOnly {
		if err := spaces.EnsureDraftSpace(ctx, d.space, d.did); err != nil {
			return nil, fmt.Errorf("ensure personal space: %w", err)
		}
	}
	metadata := draftMetadata(rec)
	operational := *rec
	setDraftMetadata(&operational, &placestream.VodDraftMetadata{})
	// Persist the operational anchor first. If the response is lost, the
	// record is visible only if it actually exists on the PDS; never retry it.
	dv, err := d.state.insertDraft(ctx, d.did, uploadID, draftInSpace, "", &operational)
	if err != nil {
		return nil, err
	}
	created, err := spaces.CreateDraftMetadata(ctx, d.space, d.did, draftRkey(dv.URI), &metadata)
	if err != nil {
		return nil, fmt.Errorf("create private draft metadata: %w", err)
	}
	if !created {
		return nil, errors.New("private draft record already exists")
	}
	return dv, nil
}

func (d *Drafts) Get(ctx context.Context, uri string) (*DraftVideo, error) {
	dv, err := d.state.getDraft(ctx, d.did, uri)
	if err != nil {
		return nil, err
	}
	if dv == nil || (d.space != nil && dv.MetadataState != draftInSpace) {
		return nil, ErrDraftNotFound
	}
	if d.space != nil {
		record, err := spaces.GetDraftMetadata(ctx, d.space, d.did, draftRkey(uri))
		if err != nil {
			return nil, fmt.Errorf("get private draft metadata: %w", err)
		}
		if record == nil {
			return nil, ErrDraftNotFound
		}
		if err := joinDraftMetadata(dv, record); err != nil {
			return nil, err
		}
	}
	return dv, nil
}

// List uses the server's creation order, not PDS list order. The URI breaks
// ties so a page never skips drafts created at exactly the cursor timestamp.
func (d *Drafts) List(ctx context.Context, limit int, cursor string) ([]*DraftVideo, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var before time.Time
	stamp, uri, _ := strings.Cut(cursor, "|")
	if cursor != "" {
		var err error
		before, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, "", ErrDraftCursor
		}
	}
	var remote map[string]*spaces.Record
	if d.space != nil {
		records, err := spaces.ListRecords(ctx, d.space, d.did, spaces.DraftMetadataCollection)
		if err != nil {
			return nil, "", err
		}
		remote = make(map[string]*spaces.Record, len(records))
		for i := range records {
			remote[records[i].Rkey] = &records[i]
		}
	}
	var result []*DraftVideo
	batchSize := limit + 1
	for {
		q := d.state.DB.WithContext(ctx).Where("user_did = ?", d.did).Order("created_at DESC, uri DESC").Limit(batchSize)
		if d.space != nil {
			q = q.Where("metadata_state = ?", draftInSpace)
		}
		if !before.IsZero() {
			q = q.Where("(created_at < ? OR (created_at = ? AND uri < ?))", before, before, uri)
		}
		var rows []*DraftVideo
		if err := q.Find(&rows).Error; err != nil {
			return nil, "", err
		}
		for _, row := range rows {
			before, uri = row.CreatedAt, row.URI
			var record *spaces.Record
			if d.space != nil {
				record = remote[draftRkey(uri)]
				if record == nil {
					continue
				}
			}
			if len(result) == limit {
				last := result[limit-1]
				return result, last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.URI, nil
			}
			if record != nil {
				if err := joinDraftMetadata(row, record); err != nil {
					return nil, "", err
				}
			}
			result = append(result, row)
		}
		if len(rows) < batchSize {
			return result, "", nil
		}
	}
}

func (d *Drafts) Update(ctx context.Context, input *placestream.VodUpdateDraft_Input) (*DraftVideo, error) {
	dv, err := d.Get(ctx, input.Uri)
	if err != nil {
		return nil, err
	}
	rec, err := dv.Record()
	if err != nil {
		return nil, err
	}
	metadata := draftMetadata(rec)
	if input.Title != nil {
		metadata.Title = *input.Title
	}
	if input.Description != nil {
		metadata.Description = input.Description
	}
	if input.DescriptionFacets != nil {
		metadata.DescriptionFacets = input.DescriptionFacets
	}
	if input.Tags != nil {
		metadata.Tags = input.Tags
	}
	if input.Thumb != nil {
		metadata.Thumb = input.Thumb
	}
	if input.Activity != nil {
		metadata.Activity = (*placestream.VodDraftMetadata_Activity)(input.Activity)
	}
	if input.ContentWarnings != nil {
		metadata.ContentWarnings = input.ContentWarnings
	}
	if input.ContentRights != nil {
		metadata.ContentRights = input.ContentRights
	}
	setDraftMetadata(rec, &metadata)
	dv.Data, dv.CID, err = marshalDraft(rec)
	if err != nil {
		return nil, err
	}
	if d.space != nil {
		ok, err := spaces.WriteExistingDraftMetadata(ctx, d.space, d.did, draftRkey(dv.URI), &metadata)
		if err != nil {
			return nil, fmt.Errorf("update private draft metadata: %w", err)
		}
		if !ok {
			return nil, ErrDraftNotFound
		}
	} else if err := d.state.DB.WithContext(ctx).Model(dv).Updates(map[string]any{"data": dv.Data, "cid": dv.CID}).Error; err != nil {
		return nil, err
	}
	return dv, nil
}

// Delete only removes the local anchor after private deletion succeeds.
func (d *Drafts) Delete(ctx context.Context, uri string) error {
	dv, err := d.state.getDraft(ctx, d.did, uri)
	if err != nil {
		return err
	}
	if dv == nil || (d.space != nil && dv.MetadataState != draftInSpace) {
		return ErrDraftNotFound
	}
	if d.space != nil {
		ok, err := spaces.WriteExistingDraftMetadata(ctx, d.space, d.did, draftRkey(uri), nil)
		if err != nil {
			return fmt.Errorf("delete private draft metadata: %w", err)
		}
		if !ok {
			return ErrDraftNotFound
		}
	}
	return d.state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rememberLegacyDraftUpload(tx, dv); err != nil {
			return err
		}
		return tx.Delete(dv).Error
	})
}

// AttachUpload associates a new upload with a visible owned draft. Only the
// operational anchor is written; a previous upload cannot finish into it.
func (d *Drafts) AttachUpload(ctx context.Context, uri, uploadID string) error {
	dv, err := d.state.getDraft(ctx, d.did, uri)
	if err != nil {
		return err
	}
	if dv == nil || (d.space != nil && dv.MetadataState != draftInSpace) {
		return ErrDraftNotFound
	}
	if d.space != nil {
		record, err := spaces.GetDraftMetadata(ctx, d.space, d.did, draftRkey(uri))
		if err != nil {
			return err
		}
		if record == nil {
			return ErrDraftNotFound
		}
	}
	rec, err := dv.Record()
	if err != nil {
		return err
	}
	rec.Status, rec.Error, rec.Source, rec.DurationMs = "processing", nil, nil, nil
	data, cid, err := marshalDraft(rec)
	if err != nil {
		return err
	}
	return d.state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rememberLegacyDraftUpload(tx, dv); err != nil {
			return err
		}
		res := tx.Model(&Upload{}).Where("id = ? AND user_did = ?", uploadID, d.did).Update("draft_uri", uri)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(dv).Updates(map[string]any{
			"origin_upload_id": uploadID, "data": data, "cid": cid, "content_cid": "",
		}).Error
	})
}

// Older anchors predate uploads.draft_uri. Remember their link before changing
// or deleting it, so a repeated completion task does not create a replacement.
func rememberLegacyDraftUpload(tx *gorm.DB, dv *DraftVideo) error {
	if dv.OriginUploadID == "" {
		return nil
	}
	return tx.Model(&Upload{}).
		Where("id = ? AND user_did = ? AND (draft_uri IS NULL OR draft_uri = '')", dv.OriginUploadID, dv.UserDID).
		Update("draft_uri", dv.URI).Error
}
