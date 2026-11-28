package statedb

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/atprotoauth"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
)

// StationRepoClient selects a ready, non-revoked session for the station's
// actual PDS. Refresh can change the session or grants, so check them again.
func (state *StatefulDB) StationRepoClient(ctx context.Context, did, pds, collection string, actions ...string) (*oatproxy.XrpcClient, error) {
	if state.OATProxy == nil || state.DB == nil || did == "" || pds == "" {
		return nil, errors.New("station OAuth publisher is not configured")
	}
	if collection == "" || len(actions) == 0 {
		return nil, errors.New("station OAuth publisher requires a collection and actions")
	}
	var sessions []oatproxy.OAuthSession
	if err := state.DB.WithContext(ctx).
		Where("repo_did = ? AND pds_url IN ?", did, []string{pds, pds + "/"}).
		Where("revoked_at IS NULL AND downstream_revoked_at IS NULL").
		Where("downstream_dpop_jkt <> '' AND downstream_access_token <> '' AND upstream_dpop_private_jwk <> '' AND upstream_scope <> ''").
		Where("upstream_access_token_exp IS NOT NULL AND (upstream_refresh_token <> '' OR upstream_access_token_exp > ?)", time.Now().UTC()).
		Order("updated_at DESC").Find(&sessions).Error; err != nil {
		return nil, errors.New("could not load station OAuth sessions")
	}
	for i := range sessions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		session := &sessions[i]
		if !stationRepoSessionEligible(session, did, pds, collection, actions) {
			continue
		}
		// An access-only grant is usable until expiry; oatproxy's early
		// refresh would otherwise attempt to use a missing refresh token.
		if session.UpstreamRefreshToken != "" {
			var err error
			session, err = state.OATProxy.RefreshIfNeeded(session)
			if err != nil || !stationRepoSessionEligible(session, did, pds, collection, actions) {
				continue
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if session.UpstreamAccessToken == "" || !session.UpstreamAccessTokenExp.After(time.Now()) {
			continue
		}
		bound := *session
		bound.PDSUrl = pds // oatproxy appends /xrpc to the base URL
		client, err := state.OATProxy.GetXrpcClient(&bound)
		if err == nil {
			return client, nil
		}
	}
	return nil, fmt.Errorf("no usable station OAuth session with %s grants on %s", strings.Join(actions, "+"), collection)
}

func stationRepoSessionEligible(session *oatproxy.OAuthSession, did, pds, collection string, actions []string) bool {
	return session != nil && session.Status() == oatproxy.OAuthSessionStateReady &&
		session.DID == did && strings.TrimSuffix(session.PDSUrl, "/") == pds && session.DownstreamDPoPJKT != "" &&
		session.UpstreamDPoPPrivateJWK != "" && session.UpstreamAccessTokenExp != nil &&
		((session.UpstreamAccessToken != "" && session.UpstreamAccessTokenExp.After(time.Now())) ||
			(session.UpstreamRefreshToken != "" && session.UpstreamAuthServerIssuer != "")) &&
		stationRepoScopeGranted(session.UpstreamScope, collection, actions)
}

func stationRepoScopeGranted(scope, collection string, actions []string) bool {
	if len(actions) == 0 || slices.Contains(actions, "") {
		return false
	}
	full := collection == constants.PLACE_STREAM_MEDIA_ORIGIN || collection == constants.PLACE_STREAM_MEDIA_VIEW_COUNT
	for _, action := range actions {
		granted := false
		for token := range strings.FieldsSeq(scope) {
			if full && token == "include:"+lexicons.AuthFullID {
				return true
			}
			if atprotoauth.RepoGrant(token, collection, action, true) {
				granted = true
				break
			}
		}
		if !granted {
			return false
		}
	}
	return true
}

// StationOriginLabels reads the same indexed remote labels as VOD playback.
func (state *StatefulDB) StationOriginLabels(uri string) ([]*comatproto.LabelDefs_Label, error) {
	if state.model == nil {
		return nil, errors.New("station publisher has no moderation index")
	}
	return state.model.GetActiveLabels(uri)
}
