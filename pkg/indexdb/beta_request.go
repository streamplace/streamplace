package indexdb

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// BetaRequest is the indexed view of a place.stream.beta.request
// record. Unlike an invite, a request is published in the requester's
// own repo, so the requesting account *is* the record's authority —
// RepoDID is both who published it and who it's about. One row per
// (RepoDID, Feature) pair; both columns sit in the composite index
// since HasBetaRequest filters by exactly that shape.
type BetaRequest struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index:idx_requests_lookup,priority:1"`
	Feature   string    `gorm:"column:feature;index:idx_requests_lookup,priority:2"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertBetaRequest(ctx context.Context, rec placestream.BetaRequest, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get beta request CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal beta request record: %w", err)
	}
	req := &BetaRequest{
		URI:       aturi.String(),
		CID:       cid.String(),
		RepoDID:   repoDID.String(),
		Feature:   rec.Feature,
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(req).Error
}

func (m *DBModel) DeleteBetaRequest(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&BetaRequest{}).Error
}

// HasBetaRequest reports whether `subjectDID` has an outstanding
// place.stream.beta.request record on file for `feature`.
func (m *DBModel) HasBetaRequest(ctx context.Context, subjectDID, feature string) (bool, error) {
	var count int64
	err := m.DB.WithContext(ctx).
		Model(&BetaRequest{}).
		Where("repo_did = ? AND feature = ?", subjectDID, feature).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("has beta request: %w", err)
	}
	return count > 0, nil
}
