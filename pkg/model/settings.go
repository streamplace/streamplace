package model

import "fmt"

// settings for publishing from a particular key. mostly node-local.
type Settings struct {
	ID       string `json:"id" gorm:"primaryKey"`
	Streamer string `json:"streamer"`
	Title    string `json:"title"`
}

func (m *DBModel) GetSettings(id string) (*Settings, error) {
	var settings Settings
	err := m.DB.Where("id = ?", id).FirstOrCreate(&settings, Settings{
		ID: id,
	}).Error
	if err != nil {
		return nil, fmt.Errorf("error getting settings: %w", err)
	}
	return &settings, nil
}

func (m *DBModel) UpdateSettings(settings *Settings) error {
	err := m.DB.Where("id = ?", settings.ID).Save(settings).Error
	if err != nil {
		return fmt.Errorf("error updating settings: %w", err)
	}
	return nil
}
