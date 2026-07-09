package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/placestream"
)

// MediaTrack is the indexed view of a place.stream.media.track
// record. Only fields used to query — Blob (for "list tracks in this
// container") and RepoDID (for ownership checks on those query
// results) — sit beside the URI/CID identity. Everything else lives
// in the CBOR Record blob and is reached via ToRecord.
type MediaTrack struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did"`
	Blob      string    `gorm:"column:blob;index"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

// ToRecord decodes the stored CBOR into the typed lexicon struct.
func (t *MediaTrack) ToRecord() (*streamplace.MediaTrack, error) {
	rec, err := lexutil.CborDecodeValue(t.Record)
	if err != nil {
		return nil, fmt.Errorf("decode media track record: %w", err)
	}
	track, ok := rec.(*streamplace.MediaTrack)
	if !ok {
		return nil, fmt.Errorf("media track record decoded as %T, expected *streamplace.MediaTrack", rec)
	}
	return track, nil
}

// trackBlob pulls the MUXL container's blob CID off a typed track
// record. Tracks not backed by a muxlTrack (no other shape defined
// yet) return an empty string, leaving it to the caller to decide
// whether that's worth indexing.
func trackBlob(rec *streamplace.MediaTrack) string {
	if rec == nil || rec.Track == nil || rec.Track.MediaDefs_MuxlTrack == nil {
		return ""
	}
	return rec.Track.MediaDefs_MuxlTrack.Blob
}

func (m *DBModel) UpsertMediaTrack(ctx context.Context, rec *streamplace.MediaTrack, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(rec)
	if err != nil {
		return fmt.Errorf("get media track CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal media track record: %w", err)
	}
	t := &MediaTrack{
		URI:       aturi.String(),
		CID:       cid.String(),
		RepoDID:   repoDID.String(),
		Blob:      trackBlob(rec),
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(t).Error
}

func (m *DBModel) DeleteMediaTrack(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&MediaTrack{}).Error
}

func (m *DBModel) GetMediaTrackByURI(ctx context.Context, uri string) (*streamplace.MediaTrack, error) {
	var t MediaTrack
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media track by uri: %w", err)
	}
	return t.ToRecord()
}

// GetMediaTracksByBlob returns every track row that claims to live
// inside the given MUXL container, across all repos. Returns model
// rows (not decoded records) so callers have RepoDID for ownership
// checks without paying for a CBOR decode per track.
func (m *DBModel) GetMediaTracksByBlob(ctx context.Context, blob string) ([]*MediaTrack, error) {
	var out []*MediaTrack
	err := m.DB.WithContext(ctx).
		Where("blob = ?", blob).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tracks for blob: %w", err)
	}
	return out, nil
}
