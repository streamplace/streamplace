package model

import (
	"errors"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
)

func (m *DBModel) CreateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	return m.DB.Create(session).Error
}

func (m *DBModel) LoadOAuthSession(id string) (*oatproxy.OAuthSession, error) {
	var session oatproxy.OAuthSession
	if err := m.DB.Where("downstream_dpop_jkt = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (m *DBModel) UpdateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	res := m.DB.Model(&oatproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id).Updates(session)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return nil
}

func (m *DBModel) ListOAuthSessions() ([]oatproxy.OAuthSession, error) {
	var sessions []oatproxy.OAuthSession
	if err := m.DB.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (m *DBModel) GetSessionByDID(did string) (*oatproxy.OAuthSession, error) {
	var session oatproxy.OAuthSession
	if err := m.DB.Where("repo_did = ? AND revoked_at IS NULL", did).Order("updated_at DESC").First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}
