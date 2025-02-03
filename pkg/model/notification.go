package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	Token     string  `gorm:"primarykey"`
	RepoDID   *string `json:"repoDID,omitempty"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (m *DBModel) CreateNotification(token string) error {
	err := m.DB.Model(Notification{}).Create(&Notification{
		Token: token,
	}).Error
	if err != nil {
		return err
	}
	return nil
}

func (m *DBModel) ListNotifications() ([]Notification, error) {
	nots := []Notification{}
	err := m.DB.Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}
