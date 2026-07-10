package model

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

// BetaInvite is the indexed view of a place.stream.beta.invite
// record. One row per (RepoDID, DID, Feature) triple; all three sit
// in the composite index since HasBetaInvite filters by exactly that
// shape. The trust model is "we believe whoever owns the repo": at
// the gate, callers must filter by RepoDID to the operator-configured
// `--beta-invite-did` so a random user's repo can't mint invites for
// our node.
type BetaInvite struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index:idx_invites_lookup,priority:1"`
	DID       string    `gorm:"column:did;index:idx_invites_lookup,priority:2"`
	Feature   string    `gorm:"column:feature;index:idx_invites_lookup,priority:3"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertBetaInvite(ctx context.Context, rec placestream.BetaInvite, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get beta invite CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal beta invite record: %w", err)
	}
	inv := &BetaInvite{
		URI:       aturi.String(),
		CID:       cid.String(),
		RepoDID:   repoDID.String(),
		DID:       rec.Did,
		Feature:   rec.Feature,
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(inv).Error
}

func (m *DBModel) DeleteBetaInvite(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&BetaInvite{}).Error
}

// HasBetaInvite reports whether `fromRepoDID` has published a
// place.stream.beta.invite record for (subjectDID, feature).
// Callers are expected to pass the operator-configured trusted issuer
// DID for `fromRepoDID` — invites from any other repo are ignored.
func (m *DBModel) HasBetaInvite(ctx context.Context, fromRepoDID, subjectDID, feature string) (bool, error) {
	var count int64
	err := m.DB.WithContext(ctx).
		Model(&BetaInvite{}).
		Where("repo_did = ? AND did = ? AND feature = ?", fromRepoDID, subjectDID, feature).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("has beta invite: %w", err)
	}
	return count > 0, nil
}
