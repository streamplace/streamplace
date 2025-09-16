package statedb

import (
	"fmt"
	"time"
)

type Notification struct {
	Token     string    `gorm:"column:token;primarykey"`
	RepoDID   string    `json:"repoDID,omitempty" gorm:"column:repo_did;index"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (state *StatefulDB) CreateNotification(token string, repoDID string) error {
	not := Notification{
		Token: token,
	}
	if repoDID != "" {
		not.RepoDID = repoDID
	}
	err := state.DB.Save(&not).Error
	if err != nil {
		return err
	}
	return nil
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
