package statedb

import (
	"time"
)

type BroadcastOrigin struct {
	StreamerRepoDID string    `gorm:"column:streamer_repo_did;primarykey;index:idx_streamer_repo_did_updated_at,priority:1"`
	ServerDID       string    `gorm:"column:server_did;primarykey;index:idx_server_did_updated_at,priority:1"`
	UpdatedAt       time.Time `gorm:"column:updated_at;index:idx_streamer_repo_did_updated_at,priority:2;index:idx_server_did_updated_at,priority:2"`
}

func (m *BroadcastOrigin) TableName() string {
	return "broadcast_origins"
}

// UpsertBroadcastOrigin inserts or updates a BroadcastOrigin entry.
// If an entry with the same StreamerRepoDID and ServerRepoDID exists, it updates UpdatedAt.
// Otherwise, it creates a new entry.
func (state *StatefulDB) UpsertBroadcastOrigin(streamerRepoDID, serverRepoDID string, updatedAt time.Time) error {
	broadcastOrigin := &BroadcastOrigin{
		StreamerRepoDID: streamerRepoDID,
		ServerDID:       serverRepoDID,
		UpdatedAt:       updatedAt,
	}
	// Uses GORM's upsert ("ON CONFLICT DO UPDATE") by providing primary keys and using Updates
	return state.DB.
		Clauses(
		// GORM uses these settings to upsert
		// The clause 'ON CONFLICT (primary key) DO UPDATE' is default when calling Save
		).
		Save(broadcastOrigin).Error
}

// GetLatestBroadcastOriginForStreamer retrieves the most recent BroadcastOrigin for a given streamerRepoDID,
// ordered by UpdatedAt descending, and returns the first found.
func (state *StatefulDB) GetLatestBroadcastOriginForStreamer(streamerRepoDID string) (*BroadcastOrigin, error) {
	var origin BroadcastOrigin
	tx := state.DB.
		Where("streamer_repo_did = ?", streamerRepoDID).
		Order("updated_at DESC").
		Limit(1).
		Find(&origin)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}
	return &origin, nil
}

// ListBroadcastOriginsSince returns every streamer→server row refreshed
// after since, newest first. With one statedb shared by a station this is
// how peers learn which node is ingesting whom — the ingest node touches
// its row on every segment.
func (state *StatefulDB) ListBroadcastOriginsSince(since time.Time) ([]BroadcastOrigin, error) {
	var rows []BroadcastOrigin
	err := state.DB.Where("updated_at >= ?", since.UTC()).Order("updated_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
