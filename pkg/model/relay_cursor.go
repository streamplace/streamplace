package model

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RelayCursor remembers how far we have consumed each relay's firehose, keyed by
// the relay's websocket URL. On reconnect or restart we resume from the stored
// sequence number instead of re-tailing from live (which would leave a gap) or
// replaying from the beginning. Cursors are per-relay because each relay
// assigns its own sequence numbers.
type RelayCursor struct {
	Host   string `gorm:"primaryKey;column:host"`
	Cursor int64  `gorm:"column:cursor"`
}

// GetRelayCursor returns the stored cursor for a relay, or nil if we have never
// recorded one (i.e. this is a fresh subscription).
func (m *DBModel) GetRelayCursor(host string) (*RelayCursor, error) {
	var rc RelayCursor
	err := m.DB.Where("host = ?", host).First(&rc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// UpsertRelayCursor stores the latest consumed sequence number for a relay.
func (m *DBModel) UpsertRelayCursor(host string, cursor int64) error {
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "host"}},
		DoUpdates: clause.AssignmentColumns([]string{"cursor"}),
	}).Create(&RelayCursor{Host: host, Cursor: cursor}).Error
}
