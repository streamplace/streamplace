package statedb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"stream.place/streamplace/pkg/stationdb"
)

// Upload tracks a TUS upload and the results of VOD processing. Station TUS
// offsets are stored separately so standalone uploads never match station CAS.
type Upload struct {
	// ID is the TUS upload identifier (also embedded in the bearer token).
	ID          string     `gorm:"column:id;primarykey"`
	RepoDID     string     `gorm:"column:user_did;index;not null"`
	MimeType    string     `gorm:"column:mime_type"`
	Filename    string     `gorm:"column:filename"`
	Size        int64      `gorm:"column:size"`
	Backend     string     `gorm:"column:backend"` // "file" or "s3"
	Location    string     `gorm:"column:location"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	// Retained after draft deletion or re-upload so repeated TUS completion
	// cannot mint a replacement draft for this upload.
	DraftURI string `gorm:"column:draft_uri"`

	// Processing fields — set by the VOD pipeline after the TUS upload finishes.
	// ProcessingStatus is "", "processing", "done", or "error".
	ProcessingStatus   string `gorm:"column:processing_status"`
	ProcessingError    string `gorm:"column:processing_error"`
	ProcessingProgress int    `gorm:"column:processing_progress;default:0"`
	// TrackURIs is a JSON array of {"uri":"at://...","cid":"..."} objects.
	// Vestigial since track publication was deferred to publishDraft time:
	// new uploads leave this empty and PublishDraft publishes the tracks on
	// demand. Retained so existing rows / the legacy publishVideo path still
	// read it.
	TrackURIs  string `gorm:"column:track_uris"`
	DurationMS int64  `gorm:"column:duration_ms"`
	// ContentCID is the BDASL CID of the processed fMP4 blob. Stored so the
	// server can locate the content blob + metafile later (e.g. for
	// publishVideo's thumbnail generation) without re-deriving it from the
	// published track records.
	ContentCID string `gorm:"column:content_cid"`
	// SigningKey is the did:key whose ephemeral private half C2PA-signed the
	// segments. Stored at processing time so PublishDraft can publish the
	// place.stream.media.track records (which carry it) at publish time.
	SigningKey string `gorm:"column:signing_key"`
	// ProbeJSON is the gstreamer probe metadata (video/audio codec, dims,
	// fps, rate, channels) serialized as JSON. Stored at processing time so
	// PublishDraft can publish the track records (deferred from processing)
	// without re-probing the blob.
	ProbeJSON string `gorm:"column:probe_json"`
	// BlobSize is the byte size of the processed MUXL content blob (distinct
	// from Size, the raw upload size). Stored at processing time so
	// PublishDraft can populate the track records' size field without
	// re-statting the blob.
	BlobSize int64 `gorm:"column:blob_size"`
}

func (Upload) TableName() string {
	return "uploads"
}

func (state *StatefulDB) CreateUpload(ctx context.Context, u *Upload) error {
	return state.DB.WithContext(ctx).Create(u).Error
}

func (state *StatefulDB) GetUpload(ctx context.Context, id string) (*Upload, error) {
	var u Upload
	err := state.DB.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (state *StatefulDB) CompleteUpload(ctx context.Context, id string, location string) error {
	now := time.Now().UTC()
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"completed_at": &now,
			"location":     location,
		}).Error
}

func (state *StatefulDB) SetUploadProcessing(ctx context.Context, id string) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status":   "processing",
			"processing_progress": 0,
		}).Error
}

func (state *StatefulDB) SetUploadProgress(ctx context.Context, id string, progress int) error {
	return state.withTaskWrite(ctx, func(db *StatefulDB) error {
		return db.DB.WithContext(ctx).Model(&Upload{}).
			Where("id = ?", id).
			Update("processing_progress", progress).Error
	})
}

// SetUploadProcessed marks an upload done and stores the processing results
// the publishDraft path needs: duration, the content blob's CID, the C2PA
// signing key, and the gstreamer probe metadata (JSON). Track records are
// NOT published here — they're deferred to publishDraft time so half-
// published tracks don't go live before the video record. trackURIs is left
// empty (vestigial; the legacy publishVideo path may still set it).
func (state *StatefulDB) SetUploadProcessed(ctx context.Context, id string, durationMS int64, contentCID, signingKey, probeJSON string, blobSize int64) error {
	return state.withTaskWrite(ctx, func(db *StatefulDB) error {
		return db.DB.WithContext(ctx).Model(&Upload{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"processing_status":   "done",
				"processing_error":    "",
				"processing_progress": 100,
				"duration_ms":         durationMS,
				"content_cid":         contentCID,
				"signing_key":         signingKey,
				"probe_json":          probeJSON,
				"blob_size":           blobSize,
			}).Error
	})
}

// SetUploadTrackURIs remembers the published place.stream.media.track records
// of a finished upload (JSON array of {"uri","cid"}), so a later publish of the
// same upload reuses them instead of publishing a second set.
func (state *StatefulDB) SetUploadTrackURIs(ctx context.Context, id string, trackURIs string) error {
	return state.withTaskWrite(ctx, func(db *StatefulDB) error {
		return db.DB.WithContext(ctx).Model(&Upload{}).Where("id = ?", id).Updates(map[string]any{
			"track_uris": trackURIs,
			"updated_at": time.Now(),
		}).Error
	})
}

// ForgetUploadTracks clears the remembered track records of every upload
// of the repo that references one of the given (now deleted) track URIs, so
// the next publish from such an upload mints new track records instead of
// pointing at deleted ones.
func (state *StatefulDB) ForgetUploadTracks(ctx context.Context, did string, deleted []string) error {
	uploads, err := state.ListUploadsForRepo(ctx, did)
	if err != nil {
		return err
	}
	gone := map[string]bool{}
	for _, u := range deleted {
		gone[u] = true
	}
	for _, u := range uploads {
		if u.TrackURIs == "" {
			continue
		}
		var refs []struct{ URI string }
		if err := json.Unmarshal([]byte(u.TrackURIs), &refs); err != nil {
			continue
		}
		for _, r := range refs {
			if gone[r.URI] {
				if err := state.SetUploadTrackURIs(ctx, u.ID, ""); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// ListUploadsForRepo lists a repo's uploads, newest first.
func (state *StatefulDB) ListUploadsForRepo(ctx context.Context, did string) ([]Upload, error) {
	var out []Upload
	err := state.DB.WithContext(ctx).Where("user_did = ?", did).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (state *StatefulDB) SetUploadFailed(ctx context.Context, id string, errMsg string) error {
	return state.DB.WithContext(ctx).Model(&Upload{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"processing_status": "error",
			"processing_error":  errMsg,
		}).Error
}

type stationUploadCursor struct {
	UploadID string `gorm:"column:upload_id;primaryKey"`
	Offset   int64  `gorm:"column:byte_offset"`
}

func (stationUploadCursor) TableName() string { return "station_upload_cursors" }

// StationUpload is the upload metadata and its authoritative TUS offset,
// read in one SQL statement.
type StationUpload struct {
	Upload
	Offset int64 `gorm:"column:byte_offset"`
}

// StationUploadPart points to one complete, immutable PATCH object.
type StationUploadPart struct {
	UploadID  string `gorm:"column:upload_id;primaryKey"`
	Offset    int64  `gorm:"column:byte_offset;primaryKey;autoIncrement:false"`
	Size      int64  `gorm:"column:size"`
	ObjectKey string `gorm:"column:object_key"`
}

func (StationUploadPart) TableName() string { return "station_upload_parts" }

var ErrStationUploadOffset = errors.New("station upload: offset changed")

// MigrateStationUploads installs the station-only models. IF NOT EXISTS
// avoids a check-then-create race when multiple stations boot together.
func (state *StatefulDB) MigrateStationUploads(ctx context.Context) error {
	return state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS station_upload_cursors (
			upload_id TEXT NOT NULL PRIMARY KEY,
			byte_offset INTEGER NOT NULL CHECK (byte_offset >= 0)
		)`).Error; err != nil {
			return err
		}
		return tx.Exec(`CREATE TABLE IF NOT EXISTS station_upload_parts (
			upload_id TEXT NOT NULL,
			byte_offset INTEGER NOT NULL CHECK (byte_offset >= 0),
			size INTEGER NOT NULL CHECK (size > 0),
			object_key TEXT NOT NULL,
			PRIMARY KEY (upload_id, byte_offset)
		)`).Error
	})
}

func (state *StatefulDB) CreateStationUpload(ctx context.Context, upload *Upload) error {
	return state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(upload).Error; err != nil {
			return err
		}
		return tx.Create(&stationUploadCursor{UploadID: upload.ID}).Error
	})
}

func (state *StatefulDB) GetStationUpload(ctx context.Context, id string) (*StationUpload, error) {
	var upload StationUpload
	err := state.DB.WithContext(ctx).Table("station_upload_cursors AS cur").
		Joins("JOIN uploads ON uploads.id = cur.upload_id").
		Select("uploads.id, uploads.user_did, uploads.mime_type, uploads.filename, uploads.size, uploads.backend, uploads.location, cur.byte_offset").
		Where("cur.upload_id = ?", id).
		Take(&upload).Error
	if err != nil {
		return nil, err
	}
	return &upload, nil
}

// ListStationUploadParts reads one page without pinning a DB snapshot during
// object I/O. Committed parts are immutable and are only reclaimed offline.
func (state *StatefulDB) ListStationUploadParts(ctx context.Context, id string, start, end int64) ([]StationUploadPart, error) {
	var parts []StationUploadPart
	err := state.DB.WithContext(ctx).
		Where("upload_id = ? AND byte_offset >= ? AND byte_offset < ?", id, start, end).
		Order("byte_offset").Limit(128).Find(&parts).Error
	return parts, err
}

func casStationUploadOffset(tx *gorm.DB, id string, from, to int64) error {
	res := tx.Model(&stationUploadCursor{}).Where("upload_id = ? AND byte_offset = ?", id, from).
		Update("byte_offset", to)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var cursor stationUploadCursor
		if err := tx.Take(&cursor, "upload_id = ?", id).Error; err != nil {
			return err
		}
		return ErrStationUploadOffset
	}
	return nil
}

func (state *StatefulDB) AppendStationUploadPart(ctx context.Context, part StationUploadPart) error {
	return state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := casStationUploadOffset(tx, part.UploadID, part.Offset, part.Offset+part.Size); err != nil {
			return err
		}
		return tx.Create(&part).Error
	})
}

// CompleteStationUpload publishes the candidate and its VOD task atomically.
// The existing task queue is the outbox; there is no post-commit delivery step.
func (state *StatefulDB) CompleteStationUpload(ctx context.Context, upload *StationUpload, location string) error {
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := casStationUploadOffset(tx, upload.ID, upload.Offset, upload.Size); err != nil {
			return err
		}

		now := time.Now().UTC()
		res := tx.Model(&Upload{}).Where("id = ?", upload.ID).
			Updates(map[string]any{"completed_at": now, "location": location})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		if _, err := state.CreateUploadDraftTx(ctx, tx, upload.RepoDID, upload.ID, nil); err != nil {
			return err
		}

		payload, err := json.Marshal(VODProcessTask{
			UploadID: upload.ID,
			RepoDID:  upload.RepoDID,
			MimeType: upload.MimeType,
			Filename: upload.Filename,
			Size:     upload.Size,
			Backend:  upload.Backend,
			Location: location,
		})
		if err != nil {
			return fmt.Errorf("marshal station VOD task: %w", err)
		}
		taskKey := "vod-process:" + upload.ID
		return tx.Create(&AppTask{
			Type:    TaskVODProcess,
			TaskKey: &taskKey,
			Status:  TaskStatusPending,
			Payload: payload,
		}).Error
	})
	if err == nil {
		select {
		case state.pokeQueue <- struct{}{}:
		default:
		}
	}
	return err
}

// TerminateStationUpload removes the TUS resource and fences writers. Keep
// the Upload row and any final object: a durable VOD task may still need them.
func (state *StatefulDB) TerminateStationUpload(ctx context.Context, id string) error {
	res := state.DB.WithContext(ctx).Where("upload_id = ?", id).Delete(&stationUploadCursor{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// uploadAuthKeySize is the size in bytes of the HMAC key used to sign upload
// bearer tokens. 32 bytes is the recommended size for HS256.
const uploadAuthKeySize = 32

const uploadAuthKeyConfigKey = "upload-auth-key"

// GetOrCreateUploadAuthKey returns the symmetric HMAC key used to sign upload
// bearer tokens, lazily generating one on first call. The key is persisted in
// the stateful database so all nodes in a station agree.
func (state *StatefulDB) GetOrCreateUploadAuthKey() ([]byte, error) {
	existing, err := state.GetConfig(uploadAuthKeyConfigKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing.Value, nil
	}
	key := make([]byte, uploadAuthKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	ctx := context.Background()
	value, err := state.createSharedConfig(ctx, uploadAuthKeyConfigKey, key)
	if errors.Is(err, stationdb.ErrConflict) {
		// The insert definitely rolled back; the helper preserves any winner.
		return state.createSharedConfig(ctx, uploadAuthKeyConfigKey, key)
	}
	return value, err
}
