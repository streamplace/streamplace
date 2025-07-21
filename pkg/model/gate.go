package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type Gate struct {
	RKey          string    `gorm:"primaryKey;column:rkey"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo          *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	HiddenMessage string    `gorm:"column:hidden_message" json:"hiddenMessage"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (g *Gate) ToStreamplaceGate() (*streamplace.ChatGate, error) {
	return &streamplace.ChatGate{
		LexiconTypeID: "place.stream.chat.gate",
		HiddenMessage: g.HiddenMessage,
	}, nil
}

func (m *DBModel) CreateGate(ctx context.Context, gate *Gate) error {
	return m.DB.Create(gate).Error
}

func (m *DBModel) GetGate(ctx context.Context, rkey string) (*Gate, error) {
	var gate Gate
	err := m.DB.Preload("Repo").Where("rkey = ?", rkey).First(&gate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &gate, nil
}

func (m *DBModel) DeleteGate(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&Gate{}).Error
}

func (m *DBModel) GetUserGates(ctx context.Context, userDID string) ([]*Gate, error) {
	var gates []*Gate
	err := m.DB.Where("repo_did = ?", userDID).Find(&gates).Error
	if err != nil {
		return nil, err
	}
	return gates, nil
}
