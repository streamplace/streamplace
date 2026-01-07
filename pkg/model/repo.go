package model

import (
	"errors"

	"gorm.io/gorm"
)

type Repo struct {
	DID     string `gorm:"primaryKey;column:did" json:"did"`
	Handle  string `gorm:"index" json:"handle"`
	PDS     string `json:"pds"`
	Version string `json:"version"`
	RootCID string `json:"rootCid"`
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

func (m *DBModel) GetAllRepos() ([]Repo, error) {
	var repos []Repo
	res := m.DB.Find(&repos)
	if res.Error != nil {
		return nil, res.Error
	}
	return repos, nil
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

func (m *DBModel) GetRepoBySigningKey(signingKey string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("signing_key = ?", signingKey).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetRepoByHandleOrDID(arg string) (*Repo, error) {
	repo, err := m.GetRepoByHandle(arg)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}
	return m.GetRepo(arg)
}

func (m *DBModel) UpdateRepo(repo *Repo) error {
	return m.DB.Save(repo).Error
}

func (m *DBModel) SearchReposByHandle(query string, limit int) ([]Repo, error) {
	var repos []Repo
	// Search for repos where handle starts with the query (case-insensitive)
	// Use LIKE with LOWER for sqlite/postgres compatibility
	res := m.DB.Where("LOWER(handle) LIKE LOWER(?)", query+"%").Limit(limit).Find(&repos)
	if res.Error != nil {
		return nil, res.Error
	}
	return repos, nil
}
