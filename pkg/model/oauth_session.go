package model

import (
	"errors"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/oproxy"
)

func (m *DBModel) CreateOAuthSession(id string, session *oproxy.OAuthSession) error {
	return m.DB.Create(session).Error
}

func (m *DBModel) LoadOAuthSession(id string) (*oproxy.OAuthSession, error) {
	var session oproxy.OAuthSession
	if err := m.DB.Where("downstream_dpop_jkt = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (m *DBModel) UpdateOAuthSession(id string, session *oproxy.OAuthSession) error {
	res := m.DB.Model(&oproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id).Updates(session)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return nil
}

func (m *DBModel) ListOAuthSessions() ([]oproxy.OAuthSession, error) {
	var sessions []oproxy.OAuthSession
	if err := m.DB.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}
