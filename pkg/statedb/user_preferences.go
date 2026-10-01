package statedb

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	placestream "stream.place/streamplace/pkg/placestream"
)

// UserPreferences are a user's private preferences on this node. Unlike the
// place.stream.server.settings record, they are not in the user's repo: the
// node keeps them here and serves them over place.stream.server.getPreferences
// and putPreferences.
type UserPreferences struct {
	RepoDID string `gorm:"column:repo_did;primarykey"`
	// AutoPublishVODs publishes the VOD of each of the user's recorded
	// livestreams as soon as the livestream ends. Set it with
	// SetAutoPublishVODs.
	AutoPublishVODs bool `gorm:"column:auto_publish_vods;not null;default:false"`
	// AutoPublishVODsSince is when AutoPublishVODs was last turned on: only
	// livestreams that end after it are published, never older recordings
	// the node still holds. Nil while it is off.
	AutoPublishVODsSince *time.Time `gorm:"column:auto_publish_vods_since"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

// SetAutoPublishVODs turns automatic VOD publishing on or off, as of now.
func (p *UserPreferences) SetAutoPublishVODs(on bool, now time.Time) {
	if on == p.AutoPublishVODs {
		return
	}
	p.AutoPublishVODs = on
	p.AutoPublishVODsSince = nil
	if on {
		p.AutoPublishVODsSince = &now
	}
}

func (p *UserPreferences) TableName() string {
	return "user_preferences"
}

// GetUserPreferences returns repoDID's preferences, or the defaults if they
// have never set any.
func (state *StatefulDB) GetUserPreferences(ctx context.Context, repoDID string) (*UserPreferences, error) {
	var prefs UserPreferences
	err := state.DB.WithContext(ctx).Where("repo_did = ?", repoDID).First(&prefs).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &UserPreferences{RepoDID: repoDID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

// PutUserPreferences creates or replaces a user's preferences.
func (state *StatefulDB) PutUserPreferences(ctx context.Context, prefs *UserPreferences) error {
	return state.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_did"}},
		DoUpdates: clause.AssignmentColumns([]string{"auto_publish_vods", "auto_publish_vods_since", "updated_at"}),
	}).Create(prefs).Error
}

func (p *UserPreferences) ToLexicon() placestream.ServerDefs_Preferences {
	return placestream.ServerDefs_Preferences{
		AutoPublishVods: p.AutoPublishVODs,
	}
}
