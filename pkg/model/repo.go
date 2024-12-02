package model

import (
	"errors"

	"gorm.io/gorm"
)

type Repo struct {
	DID         string `gorm:"primaryKey;column:did" json:"did"`
	Handle      string `gorm:"index" json:"handle"`
	PDS         string `json:"pds"`
	Version     string `json:"version"`
	AquareumKey string `gorm:"index" json:"aquareumKey"`
	RootCID     string `json:"rootCid"`
}

func (Repo) TableName() string {
	return "repos"
}

func (m *DBModel) GetRepo(did string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("did = ?", did).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetRepoByHandle(handle string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("handle = ?", handle).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetRepoByAquareumKey(aquareumKey string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("aquareum_key = ?", aquareumKey).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) UpdateRepo(repo *Repo) error {
	return m.DB.Save(repo).Error
}
