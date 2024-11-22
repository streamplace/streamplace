package model

import (
	"errors"

	"gorm.io/gorm"
)

type Repo struct {
	DID         string `gorm:"primaryKey;column:did"`
	Handle      string `gorm:"index"`
	PDS         string
	Version     string
	AquareumKey string
	RootCID     string
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

func (m *DBModel) UpdateRepo(repo *Repo) error {
	return m.DB.Save(repo).Error
}
