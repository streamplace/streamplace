package model

import (
	"errors"

	"gorm.io/gorm"
)

type Labeler struct {
	DID    string `gorm:"primaryKey;column:did"`
	Cursor int64  `gorm:"column:cursor"`
}

func (m *DBModel) GetLabeler(did string) (*Labeler, error) {
	var labeler Labeler
	err := m.DB.Where("did = ?", did).First(&labeler).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &labeler, nil
}

func (m *DBModel) CreateLabeler(did string) (*Labeler, error) {
	labeler := &Labeler{
		DID:    did,
		Cursor: 0,
	}

	if err := m.DB.Create(labeler).Error; err != nil {
		return nil, err
	}
	return labeler, nil
}

func (m *DBModel) UpdateLabelerCursor(did string, cursor int64) error {
	return m.DB.Model(&Labeler{}).Where("did = ?", did).Update("cursor", cursor).Error
}
