package oproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

var refreshWhenRemaining = time.Minute * 15

// OAuthSession stores authentication data needed during the OAuth flow
type OAuthSession struct {
	DID    string `json:"did" gorm:"column:repo_did;index"`
	Handle string `json:"handle" gorm:"column:handle;index"` // possibly also did if they have no handle
	PDSUrl string `json:"pds_url" gorm:"column:pds_url;index"`

	// Upstream fields
	UpstreamState            string     `json:"upstream_state" gorm:"column:upstream_state;index"`
	UpstreamAuthServerIssuer string     `json:"upstream_auth_server_issuer" gorm:"column:upstream_auth_server_issuer"`
	UpstreamPKCEVerifier     string     `json:"upstream_pkce_verifier" gorm:"column:upstream_pkce_verifier"`
	UpstreamDPoPNonce        string     `json:"upstream_dpop_nonce" gorm:"column:upstream_dpop_nonce"`
	UpstreamDPoPPrivateJWK   string     `json:"upstream_dpop_private_jwk" gorm:"column:upstream_dpop_private_jwk;type:text"`
	UpstreamAccessToken      string     `json:"upstream_access_token" gorm:"column:upstream_access_token"`
	UpstreamAccessTokenExp   *time.Time `json:"upstream_access_token_exp" gorm:"column:upstream_access_token_exp"`
	UpstreamRefreshToken     string     `json:"upstream_refresh_token" gorm:"column:upstream_refresh_token"`

	// Downstream fields
	DownstreamDPoPNonce         string     `json:"downstream_dpop_nonce" gorm:"column:downstream_dpop_nonce"`
	DownstreamDPoPJKT           string     `json:"downstream_dpop_jkt" gorm:"column:downstream_dpop_jkt;primaryKey"`
	DownstreamAccessToken       string     `json:"downstream_access_token" gorm:"column:downstream_access_token;index"`
	DownstreamRefreshToken      string     `json:"downstream_refresh_token" gorm:"column:downstream_refresh_token;index"`
	DownstreamAuthorizationCode string     `json:"downstream_authorization_code" gorm:"column:downstream_authorization_code;index"`
	DownstreamState             string     `json:"downstream_state" gorm:"column:downstream_state"`
	DownstreamScope             string     `json:"downstream_scope" gorm:"column:downstream_scope"`
	DownstreamCodeChallenge     string     `json:"downstream_code_challenge" gorm:"column:downstream_code_challenge"`
	DownstreamPARRequestURI     string     `json:"downstream_par_request_uri" gorm:"column:downstream_par_request_uri"`
	DownstreamPARUsedAt         *time.Time `json:"downstream_par_used_at" gorm:"column:downstream_par_used_at"`
	DownstreamRedirectURI       string     `json:"downstream_redirect_uri" gorm:"column:downstream_redirect_uri"`

	RevokedAt *time.Time `json:"revoked_at" gorm:"column:revoked_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// for gorm. this is prettier than "o_auth_sessions"
func (o *OAuthSession) TableName() string {
	return "oauth_sessions"
}

type OAuthSessionStatus string

const (
	// We've gotten the first request and sent it back for a new nonce
	OAuthSessionStatePARPending OAuthSessionStatus = "par-pending"
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
	if o.DownstreamDPoPNonce != "" {
		return OAuthSessionStatePARPending
	}
	bs, _ := json.Marshal(o)
	fmt.Printf("unknown oauth session status: %s\n", string(bs))
	// todo: this should never happen, log a warning? panic?
	return OAuthSessionStateRejected
}

func (o *OProxy) loadOAuthSession(jkt string) (*OAuthSession, error) {
	session, err := o.userLoadOAuthSession(jkt)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	if session.Status() != OAuthSessionStateReady {
		return session, nil
	}
	if session.UpstreamAccessTokenExp.Sub(time.Now()) > refreshWhenRemaining {
		return session, nil
	}

	upstreamMeta := o.GetUpstreamMetadata()

	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   o.upstreamJWK,
		ClientId:    upstreamMeta.ClientID,
		RedirectUri: upstreamMeta.RedirectURIs[0],
	})

	dpopKey, err := jwk.ParseKey([]byte(session.UpstreamDPoPPrivateJWK))
	if err != nil {
		return nil, fmt.Errorf("failed to parse upstream dpop private key: %w", err)
	}

	// refresh upstream before returning
	resp, err := oclient.RefreshTokenRequest(context.Background(), session.UpstreamRefreshToken, session.UpstreamAuthServerIssuer, session.UpstreamDPoPNonce, dpopKey)
	if err != nil {
		// revoke, probably
		o.slog.Error("failed to refresh upstream token, revoking downstream session", "error", err)
		now := time.Now()
		session.RevokedAt = &now
		err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
		if err != nil {
			o.slog.Error("after upstream token refresh, failed to revoke downstream session", "error", err)
		}
		return nil, fmt.Errorf("failed to refresh upstream token: %w", err)
	}

	exp := time.Now().Add(time.Second * time.Duration(resp.ExpiresIn)).UTC()
	session.UpstreamAccessToken = resp.AccessToken
	session.UpstreamAccessTokenExp = &exp
	session.UpstreamRefreshToken = resp.RefreshToken

	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return nil, fmt.Errorf("failed to update downstream session after upstream token refresh: %w", err)
	}

	o.slog.Debug("refreshed upstream token", "session", session.DownstreamDPoPJKT)

	return session, nil
}
