package statedb

import (
	"context"

	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/streamplace"
)

type NotificationPreference struct {
	UserDID string `gorm:"primaryKey;column:user_did"`
	RepoDID string `gorm:"primaryKey;column:repo_did"`
	Enabled bool   `gorm:"column:enabled"`
}

func (m *NotificationPreference) TableName() string {
	return "notification_preferences"
}

func (state *StatefulDB) GetNotificationPreference(ctx context.Context, userDID, repoDID string) (*streamplace.GraphNotificationPreference, error) {
	var pref NotificationPreference
	result := state.DB.Where("user_did = ? AND repo_did = ?", userDID, repoDID).First(&pref)
	if result.RowsAffected == 0 {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &streamplace.GraphNotificationPreference{
		RepoDID: pref.RepoDID,
		Enabled: pref.Enabled,
	}, nil
}

func (state *StatefulDB) SetNotificationPreference(ctx context.Context, userDID string, record *streamplace.GraphNotificationPreference) error {
	return state.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_did"}, {Name: "repo_did"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled"}),
	}).Create(&NotificationPreference{
		UserDID: userDID,
		RepoDID: record.RepoDID,
		Enabled: record.Enabled,
	}).Error
}

func (state *StatefulDB) GetOptedOutFollowerDIDs(ctx context.Context, repoDID string, followerDIDs []string) ([]string, error) {
	if len(followerDIDs) == 0 {
		return nil, nil
	}
	var prefs []NotificationPreference
	err := state.DB.Where("repo_did = ? AND user_did IN ? AND enabled = false", repoDID, followerDIDs).Find(&prefs).Error
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(prefs))
	for _, p := range prefs {
		result = append(result, p.UserDID)
	}
	return result, nil
}
