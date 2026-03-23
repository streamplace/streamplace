package model

import (
	"context"

	"gorm.io/gorm/clause"
)

type NotificationPreference struct {
	UserDID     string `gorm:"primaryKey;index:notif_user_idx;column:user_did"`
	StreamerDID string `gorm:"primaryKey;index:notif_streamer_idx;column:streamer_did"`
	Enabled     bool   `gorm:"column:enabled"`
}

func (m *DBModel) GetNotificationPreference(ctx context.Context, userDID, streamerDID string) (*NotificationPreference, error) {
	var pref NotificationPreference
	result := m.DB.Where("user_did = ? AND streamer_did = ?", userDID, streamerDID).First(&pref)
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &pref, result.Error
}

func (m *DBModel) SetNotificationPreference(ctx context.Context, userDID, streamerDID string, enabled bool) error {
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_did"}, {Name: "streamer_did"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled"}),
	}).Create(&NotificationPreference{
		UserDID:     userDID,
		StreamerDID: streamerDID,
		Enabled:     enabled,
	}).Error
}
func (m *DBModel) GetOptedOutFollowerDIDs(ctx context.Context, streamerDID string, followerDIDs []string) ([]string, error) {
	if len(followerDIDs) == 0 {
		return nil, nil
	}
	var prefs []NotificationPreference
	err := m.DB.Where("streamer_did = ? AND user_did IN ? AND enabled = false", streamerDID, followerDIDs).Find(&prefs).Error
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(prefs))
	for _, p := range prefs {
		result = append(result, p.UserDID)
	}
	return result, nil
}

