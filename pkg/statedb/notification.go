package statedb

import (
	"fmt"
	"time"

	"gorm.io/gorm/clause"
)

type Notification struct {
	Token     string    `gorm:"column:token;primarykey"`
	RepoDID   string    `json:"repoDID,omitempty" gorm:"column:repo_did;index"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// CreateNotification registers (or refreshes) a device's push token. When a
// repoDID is supplied we upsert it onto the token's row so livestream blasts
// can target the user's followers. When repoDID is empty we make sure the
// token row exists but never clobber an existing repoDID association.
//
// This deliberately avoids DB.Save(): Save issues a full-row UPDATE including
// zero-value columns, so a re-registration with no repoDID (e.g. the client
// posts before its OAuth session has restored) would blank out repo_did and
// silently drop the user from follower notifications.
func (state *StatefulDB) CreateNotification(token string, repoDID string) error {
	if repoDID != "" {
		not := Notification{Token: token, RepoDID: repoDID}
		return state.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token"}},
			DoUpdates: clause.AssignmentColumns([]string{"repo_did", "updated_at"}),
		}).Create(&not).Error
	}
	not := Notification{Token: token}
	return state.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoNothing: true,
	}).Create(&not).Error
}

func (state *StatefulDB) ListNotifications() ([]Notification, error) {
	nots := []Notification{}
	err := state.DB.Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}

func (state *StatefulDB) ListUserNotifications(userDID string) ([]Notification, error) {
	nots := []Notification{}
	err := state.DB.Where("repo_did = ?", userDID).Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}

func (state *StatefulDB) GetManyNotificationTokens(userDIDs []string) ([]string, error) {
	tokens := []string{}
	err := state.DB.Model(&Notification{}).
		Where("repo_did IN (?)", userDIDs).
		Pluck("token", &tokens).
		Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notification tokens: %w", err)
	}
	return tokens, nil
}
