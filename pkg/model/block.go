package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/appbsky"
	glexrt "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type Block struct {
	RKey       string `gorm:"primaryKey;column:rkey"`
	CID        string `gorm:"column:cid"`
	RepoDID    string `json:"repoDID"              gorm:"column:repo_did;index:idx_repo_did_subject_did,priority:1"`
	Repo       *Repo  `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	SubjectDID string `gorm:"column:subject_did;index:idx_repo_did_subject_did,priority:2"`
	Record     []byte
	CreatedAt  time.Time
}

func (b *Block) ToStreamplaceBlock() (placestream.Defs_BlockView, error) {
	if b == nil {
		return placestream.Defs_BlockView{}, fmt.Errorf("block is nil")
	}
	if b.Repo == nil {
		return placestream.Defs_BlockView{}, fmt.Errorf("block repo is nil")
	}
	if b.Record == nil {
		return placestream.Defs_BlockView{}, fmt.Errorf("block record is nil")
	}
	rec, err := glexrt.CborDecodeValue(b.Record)
	if err != nil {
		return placestream.Defs_BlockView{}, fmt.Errorf("error decoding feed post: %w", err)
	}
	block, ok := rec.(appbsky.GraphBlock)
	if !ok {
		return placestream.Defs_BlockView{}, fmt.Errorf("record is not a GraphBlock")
	}
	return placestream.Defs_BlockView{
		LexiconTypeID: "place.stream.defs#blockView",
		Blocker: appbsky.ActorDefs_ProfileViewBasic{
			Did:    b.RepoDID,
			Handle: b.Repo.Handle,
		},
		Cid:       b.CID,
		IndexedAt: b.CreatedAt.Format(time.RFC3339),
		Record:    block,
		Uri:       fmt.Sprintf(`at://%s/app.appbsky.graph.block/%s`, b.RepoDID, b.RKey),
	}, nil
}

func (m *DBModel) CreateBlock(ctx context.Context, block *Block) error {
	return m.DB.Create(block).Error
}

func (m *DBModel) DeleteBlock(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&Block{}).Error
}

func (m *DBModel) GetBlock(ctx context.Context, rkey string) (*Block, error) {
	var block Block
	err := m.DB.Preload("Repo").Where("rkey = ?", rkey).First(&block).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (m *DBModel) GetUserBlock(ctx context.Context, userDID, subjectDID string) (*Block, error) {
	var block Block
	err := m.DB.Where("repo_did = ? AND subject_did = ?", userDID, subjectDID).First(&block).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &block, nil
}
