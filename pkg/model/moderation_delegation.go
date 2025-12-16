package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ModerationDelegation struct {
	RKey           string     `gorm:"primaryKey;column:rkey"`
	CID            string     `gorm:"column:cid"`
	RepoDID        string     `json:"repoDID" gorm:"column:repo_did;index:idx_repo_moderator,priority:1"`
	Repo           *Repo      `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	ModeratorDID   string     `gorm:"column:moderator_did;index:idx_repo_moderator,priority:2;index:idx_moderator"`
	Permissions    []byte     `gorm:"column:permissions"`     // JSON array stored as bytes
	ExpirationTime *time.Time `gorm:"column:expiration_time"` // Optional expiration timestamp
	Record         []byte     `gorm:"column:record"`          // Full CBOR record
	CreatedAt      time.Time  `gorm:"column:created_at"`
	IndexedAt      time.Time  `gorm:"column:indexed_at"`
}

func (m *DBModel) CreateModerationDelegation(ctx context.Context, delegation *ModerationDelegation) error {
	return m.DB.WithContext(ctx).Create(delegation).Error
}

func (m *DBModel) DeleteModerationDelegation(ctx context.Context, rkey string) error {
	return m.DB.WithContext(ctx).Where("rkey = ?", rkey).Delete(&ModerationDelegation{}).Error
}

func (m *DBModel) GetModerationDelegation(ctx context.Context, streamerDID, moderatorDID string) (*ModerationDelegation, error) {
	var delegation ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("repo_did = ? AND moderator_did = ?", streamerDID, moderatorDID).
		Order("created_at DESC").
		First(&delegation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &delegation, nil
}

func (m *DBModel) GetModeratorDelegations(ctx context.Context, moderatorDID string) ([]*ModerationDelegation, error) {
	var delegations []*ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("moderator_did = ?", moderatorDID).
		Find(&delegations).Error
	if err != nil {
		return nil, err
	}
	return delegations, nil
}

func (m *DBModel) GetStreamerModerators(ctx context.Context, streamerDID string) ([]*ModerationDelegation, error) {
	var delegations []*ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("repo_did = ?", streamerDID).
		Find(&delegations).Error
	if err != nil {
		return nil, err
	}
	return delegations, nil
}
