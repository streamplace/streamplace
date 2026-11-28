package statedb

import (
	"time"

	"gorm.io/gorm/clause"
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
	// Only a matching DID is a no-op; preserve the first registration time.
	db := state.DB
	newRepo := Repo{
		DID:       did,
		IndexedAt: time.Now(),
	}
	for attempt := 0; ; attempt++ {
		err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "did"}}, DoNothing: true}).Create(&newRepo).Error
		if state.stationStore == nil || !retryStationWrite(db.Statement.Context, attempt, err) {
			return err
		}
	}
}
