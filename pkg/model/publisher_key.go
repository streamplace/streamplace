package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

type PublisherKey struct {
	DID       string     `gorm:"primaryKey;column:did" json:"did"`
	RepoDID   string     `gorm:"primaryKey;column:repo_did" json:"repoDID"`
	RKey      string     `gorm:"column:rkey;index" json:"rkey"`
	Repo      *Repo      `json:"repo,omitempty" gorm:"foreignKey:RepoDID;references:DID"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt"`
}

func (PublisherKey) TableName() string {
	return "publisher_keys"
}

func (m *DBModel) UpdatePublisherKey(key *PublisherKey) error {
	return m.DB.Save(key).Error
}

func (m *DBModel) GetPublisherKey(ctx context.Context, did, repoDID string) (*PublisherKey, error) {
	_, span := otel.Tracer("publisher").Start(ctx, "GetPublisherKey")
	defer span.End()
	var key PublisherKey
	res := m.DB.Model(PublisherKey{}).Where("did = ?", did).Where("repo_did = ?", repoDID).First(&key)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if key.RevokedAt != nil {
		return nil, fmt.Errorf("publisher key revoked")
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &key, nil
}

func (m *DBModel) GetPublisherKeyByRKey(ctx context.Context, rkey string) (*PublisherKey, error) {
	_, span := otel.Tracer("publisher").Start(ctx, "GetPublisherKeyByRKey")
	defer span.End()
	var key PublisherKey
	res := m.DB.Model(PublisherKey{}).Where("rkey = ?", rkey).First(&key)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if key.RevokedAt != nil {
		return nil, fmt.Errorf("publisher key revoked")
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &key, nil
}

func (m *DBModel) GetPublisherKeysForRepo(repoDID string) ([]PublisherKey, error) {
	var keys []PublisherKey
	res := m.DB.Model(PublisherKey{}).Where("repo_did = ?", repoDID).Find(&keys)
	if res.Error != nil {
		return nil, res.Error
	}
	return keys, nil
}
