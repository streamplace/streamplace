package statedb

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/log"
)

// LivestreamViewTotal is a livestream's running view count: how many
// playback sessions (HLS or WebRTC, on this station's nodes) started while
// that livestream record was the streamer's current one. It is the X-style
// "views" number beside the concurrent viewer count: cumulative,
// session-based, and gameable by anyone who cares to reload — a reasonable
// approximation, not an audited metric. One row per livestream record, kept
// after the stream ends: the replay of a livestream (a video connected to
// its record) inherits these views.
type LivestreamViewTotal struct {
	StreamerDID   string    `gorm:"column:streamer_did;primaryKey"`
	LivestreamURI string    `gorm:"column:livestream_uri;primaryKey"`
	Views         int64     `gorm:"column:views"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (LivestreamViewTotal) TableName() string { return "livestream_view_totals" }

// StreamViewTotal is the earlier shape of the same count: one row per
// streamer, reset whenever their livestream record changed, so a stream
// that ended (or a title change that made a new record) threw the count
// away. Kept so the row a node already has is carried into
// livestream_view_totals on startup (migrateStreamViewTotals); nothing
// writes it anymore.
type StreamViewTotal struct {
	StreamerDID   string    `gorm:"column:streamer_did;primaryKey"`
	LivestreamURI string    `gorm:"column:livestream_uri"`
	Views         int64     `gorm:"column:views"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (StreamViewTotal) TableName() string { return "stream_view_totals" }

// migrateStreamViewTotals carries the per-streamer rows into the
// per-livestream table, once: a row is copied only when the livestream has
// no row yet, so the running counts of a node that already moved over are
// never overwritten by the stale legacy value.
func migrateStreamViewTotals(ctx context.Context, db *gorm.DB) error {
	var legacy []StreamViewTotal
	if err := db.WithContext(ctx).Where("livestream_uri <> ''").Find(&legacy).Error; err != nil {
		return err
	}
	moved := 0
	for _, row := range legacy {
		res := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&LivestreamViewTotal{
			StreamerDID: row.StreamerDID, LivestreamURI: row.LivestreamURI, Views: row.Views, UpdatedAt: row.UpdatedAt,
		})
		if res.Error != nil {
			return res.Error
		}
		moved += int(res.RowsAffected)
	}
	if moved > 0 {
		log.Log(ctx, "carried livestream view totals into the per-livestream table", "rows", moved)
	}
	return nil
}

// AddStreamView counts one more session for streamer's livestream
// livestreamURI ("" when no record is known) and returns that livestream's
// new total.
func (state *StatefulDB) AddStreamView(ctx context.Context, streamer, livestreamURI string) (int64, error) {
	var total int64
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row LivestreamViewTotal
		err := tx.Where("streamer_did = ? AND livestream_uri = ?", streamer, livestreamURI).First(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = LivestreamViewTotal{StreamerDID: streamer, LivestreamURI: livestreamURI, Views: 1, UpdatedAt: time.Now()}
			total = 1
			return tx.Create(&row).Error
		case err != nil:
			return err
		}
		row.Views++
		row.UpdatedAt = time.Now()
		total = row.Views
		return tx.Save(&row).Error
	})
	return total, err
}

// GetStreamViewTotal returns the running view count of the streamer's
// current livestream — the one most recently counted into — and its URI;
// 0 and "" when nothing has been counted.
func (state *StatefulDB) GetStreamViewTotal(ctx context.Context, streamer string) (int64, string, error) {
	var row LivestreamViewTotal
	err := state.DB.WithContext(ctx).Where("streamer_did = ?", streamer).Order("updated_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	return row.Views, row.LivestreamURI, nil
}

// LivestreamViewTotals returns the running view count of each of the given
// livestream records; a record nothing was counted for is absent.
func (state *StatefulDB) LivestreamViewTotals(ctx context.Context, uris []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(uris) == 0 {
		return out, nil
	}
	var rows []LivestreamViewTotal
	if err := state.DB.WithContext(ctx).Where("livestream_uri IN ?", uris).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.LivestreamURI] += r.Views
	}
	return out, nil
}
