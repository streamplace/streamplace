package statedb

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Very, very basic stateful repo representation. The complex stuff
// is handled by the indexer in model. This is literally just a set
// of DIDs that we care about so that new nodes know what to index.
type Repo struct {
	DID       string    `gorm:"primaryKey;column:did"`
	IndexedAt time.Time `gorm:"column:indexed_at;index:idx_recent_repos"`
}

func (r *Repo) TableName() string {
	return "repos"
}

func (state *StatefulDB) ListRepos(limit int, offset int) ([]Repo, error) {
	var repos []Repo
	err := state.DB.
		Order("indexed_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&repos).
		Error
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func (state *StatefulDB) AddRepo(did string) error {
	// Check if repo already exists
	var existingRepo Repo
	err := state.DB.Where("did = ?", did).First(&existingRepo).Error
	if err == nil {
		// Repo already exists, do nothing
		return nil
	}

	// If error is not "record not found", return the error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Repo doesn't exist, create it
	newRepo := Repo{
		DID:       did,
		IndexedAt: time.Now(),
	}

	return state.DB.Create(&newRepo).Error
}
