package model

import (
	"context"
	"time"
)

type Block struct {
	RKey       string `gorm:"primaryKey,column:rkey"`
	RepoDID    string `gorm:"index"`
	SubjectDID string `gorm:"index"`
	Record     []byte
	CreatedAt  time.Time
}

func (m *DBModel) CreateBlock(ctx context.Context, block *Block) error {
	return m.DB.Create(block).Error
}

func (m *DBModel) DeleteBlock(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&Block{}).Error
}

func (m *DBModel) GetUserBlock(ctx context.Context, userDID, subjectDID string) (*Block, error) {
	var block Block
	err := m.DB.Where("repo_did = ? AND subject_did = ?", userDID, subjectDID).First(&block).Error
	if err != nil {
		return nil, err
	}
	return &block, nil
}
