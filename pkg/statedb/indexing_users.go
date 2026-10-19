package statedb

import (
	"context"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
)

// indexingSessions is the persisted set of successfully authenticated local
// users. Pending OAuth flows and either kind of revocation grant no indexing
// permission. An expired access token remains usable only with a refresh token;
// OAuth records refresh failures by revoking the session.
func (state *StatefulDB) indexingSessions(ctx context.Context) *gorm.DB {
	return state.DB.WithContext(ctx).Model(&oatproxy.OAuthSession{}).
		Where("repo_did <> ? AND downstream_access_token <> ?", "", "").
		Where("revoked_at IS NULL AND downstream_revoked_at IS NULL").
		Where("upstream_refresh_token <> ? OR (upstream_access_token <> ? AND upstream_access_token_exp > ?)", "", "", time.Now().UTC())
}

// IndexingDID resolves a local session's DID or handle without discovering any
// remote account. An empty DID means no eligible session exists.
func (state *StatefulDB) IndexingDID(ctx context.Context, identifier string) (string, error) {
	var did string
	err := state.indexingSessions(ctx).
		Select("repo_did").Where("repo_did = ? OR handle = ?", identifier, identifier).
		Order("updated_at DESC").Limit(1).Scan(&did).Error
	return did, err
}

// IndexingDIDs derives boot/sweep eligibility from sessions, not the set of
// repos indexed in a previous server-mode run. No separate allowlist is kept.
func (state *StatefulDB) IndexingDIDs(ctx context.Context) ([]string, error) {
	var dids []string
	err := state.indexingSessions(ctx).Distinct("repo_did").Order("repo_did").Pluck("repo_did", &dids).Error
	return dids, err
}
