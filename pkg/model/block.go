package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
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

func (b *Block) ToStreamplaceBlock() (*streamplace.Defs_BlockView, error) {
	rec, err := lexutil.CborDecodeValue(b.Record)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	block, ok := rec.(*bsky.GraphBlock)
	if !ok {
		return nil, fmt.Errorf("record is not a GraphBlock")
	}
	return &streamplace.Defs_BlockView{
		LexiconTypeID: "place.stream.defs#blockView",
		Blocker: &bsky.ActorDefs_ProfileViewBasic{
			Did:    b.RepoDID,
			Handle: b.Repo.Handle,
		},
		Cid:       b.CID,
		IndexedAt: b.CreatedAt.Format(time.RFC3339),
		Record:    block,
		Uri:       fmt.Sprintf(`at://%s/app.bsky.graph.block/%s`, b.RepoDID, b.RKey),
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
