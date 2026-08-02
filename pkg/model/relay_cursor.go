package model

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RelayCursor remembers how far we have consumed each relay's firehose, keyed by
// the relay's URL. On reconnect or restart we resume from the stored cursor
// instead of re-tailing from live (which would leave a gap) or replaying from
// the beginning. Cursors are per-relay because each relay assigns its own
// numbering.
//
// Cursor is the at-sequence for WebSocket relays and the high-water MoQ group
// sequence for moqt:// relays (resumed via SubscribeFrom). A host is one
// transport or the other, so a single opaque monotonic int64 we hand back to
// the relay covers both — we just call a moqt:// group id a "cursor" too.
type RelayCursor struct {
	Host   string `gorm:"primaryKey;column:host"`
	Cursor int64  `gorm:"column:cursor"`
	// LastEventTime is the unix-seconds timestamp of the newest firehose event
	// observed on this relay, persisted alongside Cursor. It is what lets a
	// restart decide whether the stored cursor is recent enough to replay from,
	// without knowing anything about the relay's sequence numbering.
	LastEventTime int64 `gorm:"column:last_event_time"`
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

// UpsertRelayCursor stores the latest consumed cursor for a relay (the at-seq
// for WebSocket relays, the high-water MoQ group for moqt:// relays) together
// with the timestamp of the newest event seen there, which is what makes the
// cursor's age -- and so whether it is worth replaying from at all -- knowable
// after a restart.
func (m *DBModel) UpsertRelayCursor(host string, cursor int64, lastEventTime int64) error {
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "host"}},
		DoUpdates: clause.AssignmentColumns([]string{"cursor", "last_event_time"}),
	}).Create(&RelayCursor{Host: host, Cursor: cursor, LastEventTime: lastEventTime}).Error
}
