package statedb

import (
	"context"
	"time"

	"gorm.io/gorm/clause"
)

// CDNLogPart is the ingest cursor for archived CDN access logs: one
// row per (provider, part) that some node has claimed or finished.
// Parts are immutable at the provider, so "completed" is permanent.
// Implements viewlog.IngestCursor.
type CDNLogPart struct {
	Source      string     `gorm:"column:source;primaryKey"`
	PartID      string     `gorm:"column:part_id;primaryKey"`
	ClaimedAt   time.Time  `gorm:"column:claimed_at;not null"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	Events      int        `gorm:"column:events"`
}

func (CDNLogPart) TableName() string { return "cdn_log_parts" }

// ClaimPart atomically takes (source, partID) for this caller. A
// fresh row is inserted with ON CONFLICT DO NOTHING; if the insert
// didn't land, the row exists and we may only retake it when it's
// incomplete and its claim is older than staleAfter. Both steps are
// single statements, so two nodes racing get exactly one winner on
// either SQLite or Postgres.
func (state *StatefulDB) ClaimPart(ctx context.Context, source, partID string, staleAfter time.Duration) (bool, error) {
	now := time.Now().UTC()
	db := state.DB.WithContext(ctx)
	ins := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&CDNLogPart{
		Source: source, PartID: partID, ClaimedAt: now,
	})
	if ins.Error != nil {
		return false, ins.Error
	}
	if ins.RowsAffected == 1 {
		return true, nil
	}
	upd := db.Model(&CDNLogPart{}).
		Where("source = ? AND part_id = ? AND completed_at IS NULL AND claimed_at < ?", source, partID, now.Add(-staleAfter)).
		Update("claimed_at", now)
	if upd.Error != nil {
		return false, upd.Error
	}
	return upd.RowsAffected == 1, nil
}

// CompletePart marks the part done.
func (state *StatefulDB) CompletePart(ctx context.Context, source, partID string, events int) error {
	now := time.Now().UTC()
	return state.DB.WithContext(ctx).Model(&CDNLogPart{}).
		Where("source = ? AND part_id = ?", source, partID).
		Updates(map[string]any{"completed_at": now, "events": events}).Error
}

// ReleasePart drops an in-progress claim so the next run retries.
// Completed parts are never released.
func (state *StatefulDB) ReleasePart(ctx context.Context, source, partID string) error {
	return state.DB.WithContext(ctx).
		Where("source = ? AND part_id = ? AND completed_at IS NULL", source, partID).
		Delete(&CDNLogPart{}).Error
}
