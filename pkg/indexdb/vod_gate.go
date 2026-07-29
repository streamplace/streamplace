package indexdb

import (
	"context"
	"time"

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

func (g *VodGate) ToRecord() (placestream.VodGate, error) {
	return placestream.VodGate{
		LexiconTypeID: "place.stream.vod.gate",
		HiddenComment: g.HiddenComment,
	}, nil
}

func (m *DBModel) CreateVodGate(ctx context.Context, gate *VodGate) error {
	return m.DB.Create(gate).Error
}

func (m *DBModel) DeleteVodGate(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&VodGate{}).Error
}
