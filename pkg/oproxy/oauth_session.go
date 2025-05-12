package oproxy

import (
	"encoding/json"
	"fmt"
	"time"
)

// OAuthSession stores authentication data needed during the OAuth flow
type OAuthSession struct {
	DID    string `gorm:"column:repo_did;index"`
	PDSUrl string `gorm:"column:pds_url;index"`

	// Upstream fields
	UpstreamState            string     `gorm:"column:upstream_state;index"`
	UpstreamAuthServerIssuer string     `gorm:"column:upstream_auth_server_issuer"`
	UpstreamPKCEVerifier     string     `gorm:"column:upstream_pkce_verifier"`
	UpstreamDPoPNonce        string     `gorm:"column:upstream_dpop_nonce"`
	UpstreamDPoPPrivateJWK   string     `gorm:"column:upstream_dpop_private_jwk;type:text"`
	UpstreamAccessToken      string     `gorm:"column:upstream_access_token"`
	UpstreamAccessTokenExp   *time.Time `gorm:"column:upstream_access_token_exp"`
	UpstreamRefreshToken     string     `gorm:"column:upstream_refresh_token"`

	// Downstream fields
	DownstreamDPoPNonce         string     `gorm:"column:downstream_dpop_nonce"`
	DownstreamDPoPJKT           string     `gorm:"column:downstream_dpop_jkt;primaryKey"`
	DownstreamAccessToken       string     `gorm:"column:downstream_access_token;index"`
	DownstreamRefreshToken      string     `gorm:"column:downstream_refresh_token;index"`
	DownstreamAuthorizationCode string     `gorm:"column:downstream_authorization_code;index"`
	DownstreamState             string     `gorm:"column:downstream_state"`
	DownstreamScope             string     `gorm:"column:downstream_scope"`
	DownstreamCodeChallenge     string     `gorm:"column:downstream_code_challenge"`
	DownstreamPARRequestURI     string     `gorm:"column:downstream_par_request_uri"`
	DownstreamPARUsedAt         *time.Time `gorm:"column:downstream_par_used_at"`

	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// for gorm. this is prettier than "o_auth_sessions"
func (o *OAuthSession) TableName() string {
	return "oauth_sessions"
}

type OAuthSessionStatus string

const (
	// PAR has been created, but not yet used
	OAuthSessionStatePARCreated OAuthSessionStatus = "par-created"
	// PAR has been used, but maybe upstream will fail for some reason
	OAuthSessionStatePARUsed OAuthSessionStatus = "par-used"
	// PAR has been used, we're waiting to hear back from upstream
	OAuthSessionStateUpstream OAuthSessionStatus = "upstream"
	// Upstream came back, we've issued the user a code but it hasn't been used yet
	OAuthSessionStateDownstream OAuthSessionStatus = "downstream"
	// Code has been used, everything is good
	OAuthSessionStateReady OAuthSessionStatus = "ready"
	// For any reason we're done. Revoked or expired
	OAuthSessionStateRejected OAuthSessionStatus = "rejected"
)

func (o *OAuthSession) Status() OAuthSessionStatus {
	if o.RevokedAt != nil {
		return OAuthSessionStateRejected
	}
	if o.DownstreamAccessToken != "" {
		return OAuthSessionStateReady
	}
	if o.DownstreamAuthorizationCode != "" {
		return OAuthSessionStateDownstream
	}
	if o.UpstreamDPoPPrivateJWK != "" {
		return OAuthSessionStateUpstream
	}
	if o.DownstreamPARUsedAt != nil {
		return OAuthSessionStatePARUsed
	}
	if o.DownstreamPARRequestURI != "" {
		return OAuthSessionStatePARCreated
	}
	bs, _ := json.Marshal(o)
	fmt.Printf("unknown oauth session status: %s\n", string(bs))
	// todo: this should never happen, log a warning? panic?
	return OAuthSessionStateRejected
}

// func (m *DBModel) CreateOAuthSession(session *OAuthSession) error {
// 	uu, err := uuid.NewV7()
// 	if err != nil {
// 		return err
// 	}
// 	session.ID = uu.String()
// 	return m.DB.Create(session).Error
// }

// func (m *DBModel) GetOAuthSession(state string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("id = ?", state).First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

// func (m *DBModel) GetOAuthSessionByUpstreamState(state string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("upstream_state = ?", state).Preload("DownstreamPAR").First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

// func (m *DBModel) GetOAuthSessionByDownstreamPARID(id string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("downstream_par_id = ?", id).Preload("DownstreamPAR").First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

// func (m *DBModel) GetOAuthSessionByDownstreamAccessToken(token string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("downstream_access_token = ?", token).First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

// func (m *DBModel) GetOAuthSessionByDownstreamRefreshToken(token string) (*OAuthSession, error) {
// 	var session OAuthSession
// 	err := m.DB.Where("downstream_refresh_token = ?", token).First(&session).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &session, nil
// }

// func (m *DBModel) UpdateOAuthSession(session *OAuthSession) error {
// 	return m.DB.Save(session).Error
// }

// func (m *DBModel) DeleteOAuthSession(id string) error {
// 	return m.DB.Delete(&OAuthSession{}, "id = ?", id).Error
// }
