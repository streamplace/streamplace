package indexdb

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type Gate struct {
	RKey          string    `gorm:"primaryKey;column:rkey"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo          *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	HiddenMessage string    `gorm:"column:hidden_message" json:"hiddenMessage"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (g *Gate) ToRecord() (placestream.ChatGate, error) {
	return placestream.ChatGate{
		LexiconTypeID: "place.stream.chat.gate",
		HiddenMessage: g.HiddenMessage,
	}, nil
}

func (m *DBModel) CreateGate(ctx context.Context, gate *Gate) error {
	return m.DB.Create(gate).Error
}

func (m *DBModel) GetGate(ctx context.Context, rkey string) (*placestream.ChatGate, error) {
	var gate Gate
	err := m.DB.Preload("Repo").Where("rkey = ?", rkey).First(&gate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec, err := gate.ToRecord()
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (m *DBModel) DeleteGate(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&Gate{}).Error
}
