package statedb

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// StreamViewTotal is a livestream's running view count: how many playback
// sessions (HLS or WebRTC, on this node) have started since the streamer's
// current livestream record was created. It is the X-style "views" number
// beside the concurrent viewer count: cumulative, session-based, and
// gameable by anyone who cares to reload — a reasonable approximation, not
// an audited metric. Kept in statedb so it survives restarts and is shared
// across a station's nodes; one row per streamer, reset when the
// livestream record changes.
type StreamViewTotal struct {
	StreamerDID   string    `gorm:"column:streamer_did;primaryKey"`
	LivestreamURI string    `gorm:"column:livestream_uri"`
	Views         int64     `gorm:"column:views"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (StreamViewTotal) TableName() string { return "stream_view_totals" }

// AddStreamView counts one more session for streamer's livestream
// livestreamURI ("" when no record is known), starting over when the
// livestream changed since the last count. Returns the new total.
func (state *StatefulDB) AddStreamView(ctx context.Context, streamer, livestreamURI string) (int64, error) {
	var total int64
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row StreamViewTotal
		err := tx.Where("streamer_did = ?", streamer).First(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound) || row.LivestreamURI != livestreamURI:
			row = StreamViewTotal{StreamerDID: streamer, LivestreamURI: livestreamURI, Views: 1, UpdatedAt: time.Now()}
			total = 1
			return tx.Save(&row).Error
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

// GetStreamViewTotal returns streamer's running view count and the
// livestream it belongs to; 0 and "" when nothing has been counted.
func (state *StatefulDB) GetStreamViewTotal(ctx context.Context, streamer string) (int64, string, error) {
	var row StreamViewTotal
	err := state.DB.WithContext(ctx).Where("streamer_did = ?", streamer).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	return row.Views, row.LivestreamURI, nil
}
