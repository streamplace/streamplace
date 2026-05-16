package model

import (
	"context"
	"fmt"
	"time"
)

// BetaInvite is the indexed view of a place.stream.beta.invite record.
// One row per (RepoDID, DID, Feature) triple: a single account can
// grant a single feature to a given DID at most once at a time.
//
// The trust model is "we believe whoever owns the record's repo": at
// the gate, callers must filter by RepoDID to the operator-configured
// `--beta-invite-did` so a random user's repo can't mint invites for
// our node.
type BetaInvite struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index:idx_invites_lookup,priority:1"`
	RKey      string    `gorm:"column:rkey"`
	DID       string    `gorm:"column:did;index:idx_invites_lookup,priority:2"`
	Feature   string    `gorm:"column:feature;index:idx_invites_lookup,priority:3"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertBetaInvite(ctx context.Context, v *BetaInvite) error {
	return m.DB.WithContext(ctx).Save(v).Error
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
