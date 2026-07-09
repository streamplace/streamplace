package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type VodGate struct {
	RKey          string    `gorm:"primaryKey;column:rkey"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo          *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	HiddenComment string    `gorm:"column:hidden_comment" json:"hiddenComment"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (g *VodGate) ToStreamplaceVodGate() (*streamplace.VodGate, error) {
	return &streamplace.VodGate{
		LexiconTypeID: "place.stream.vod.gate",
		HiddenComment: g.HiddenComment,
	}, nil
}

func (m *DBModel) CreateVodGate(ctx context.Context, gate *VodGate) error {
	return m.DB.Create(gate).Error
}

func (m *DBModel) GetVodGate(ctx context.Context, rkey string) (*VodGate, error) {
	var gate VodGate
	err := m.DB.Preload("Repo").Where("rkey = ?", rkey).First(&gate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &gate, nil
}

func (m *DBModel) DeleteVodGate(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&VodGate{}).Error
}

func (m *DBModel) GetUserVodGates(ctx context.Context, userDID string) ([]*VodGate, error) {
	var gates []*VodGate
	err := m.DB.Where("repo_did = ?", userDID).Find(&gates).Error
	if err != nil {
		return nil, err
	}
	return gates, nil
}
