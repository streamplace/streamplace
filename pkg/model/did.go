package model

import (
	"errors"

	"gorm.io/gorm"
)

type DID struct {
	DID         string `gorm:"primaryKey;column:did"`
	PDS         string
	Version     string
	AquareumKey string
	RootCID     string
}

func (DID) TableName() string {
	return "dids"
}

func (m *DBModel) GetDID(did string) (*DID, error) {
	var didModel DID
	res := m.DB.Where("did = ?", did).First(&didModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &didModel, nil
}

func (m *DBModel) UpdateDID(did *DID) error {
	return m.DB.Save(did).Error
}
