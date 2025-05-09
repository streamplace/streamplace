package model

import (
	"time"

	"gorm.io/gorm"
)

// OAuthSessionUpstream stores authentication data needed during the OAuth flow
type OAuthSessionUpstream struct {
	// ID               string `gorm:"primarykey"`
	State            string    `gorm:"column:state;primarykey"`
	RepoDID          string    `gorm:"column:repo_did;index"`
	PDSUrl           string    `gorm:"column:pds_url"`
	AuthServerIssuer string    `gorm:"column:auth_server_issuer"`
	PKCEVerifier     string    `gorm:"column:pkce_verifier"`
	DPoPNonce        string    `gorm:"column:dpop_nonce"`
	DPoPPrivateJWK   []byte    `gorm:"column:dpop_private_jwk;type:text"`
	AccessToken      string    `gorm:"column:access_token"`
	AccessTokenExp   time.Time `gorm:"column:access_token_exp"`
	RefreshToken     string    `gorm:"column:refresh_token"`
	DownstreamPARID  string    `gorm:"column:downstream_par_id"`
	DownstreamPAR    *PAR      `gorm:"foreignKey:DownstreamPARID"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (m *DBModel) CreateOAuthSessionUpstream(session *OAuthSessionUpstream) error {
	return m.DB.Create(session).Error
}

func (m *DBModel) GetOAuthSessionUpstreamByState(state string) (*OAuthSessionUpstream, error) {
	var session OAuthSessionUpstream
	err := m.DB.Where("state = ?", state).Preload("DownstreamPAR").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// func (m *DBModel) GetOAuthSessionByID(id string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("id = ?", id).First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

func (m *DBModel) UpdateOAuthSessionUpstream(session *OAuthSessionUpstream) error {
	return m.DB.Save(session).Error
}

func (m *DBModel) DeleteOAuthSessionUpstream(state string) error {
	return m.DB.Delete(&OAuthSessionUpstream{}, "state = ?", state).Error
}
