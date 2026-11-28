package statedb

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/stationdb"
)

func (state *StatefulDB) CreateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	if state.stationStore == nil {
		return state.DB.Create(session).Error
	}
	for attempt := 0; ; attempt++ {
		submitted := *session
		err := state.DB.Create(&submitted).Error
		if err == nil {
			*session = submitted
			return nil
		}
		if !retryStationWrite(state.DB.Statement.Context, attempt, err) {
			return err
		}
	}
}

func (state *StatefulDB) LoadOAuthSession(id string) (*oatproxy.OAuthSession, error) {
	var session oatproxy.OAuthSession
	if err := state.DB.Where("downstream_dpop_jkt = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if session.Status() != oatproxy.OAuthSessionStateReady && session.Status() != oatproxy.OAuthSessionStateReadyUpstream {
		return &session, nil
	}
	// A delegated session is bound to the station, not the user's actual PDS.
	// The real PDS remains authoritative in the indexed repo.
	if state.CLI != nil && state.CLI.UpstreamStation != "" {
		return &session, nil
	}
	r, err := state.model.GetRepo(session.DID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	if r == nil {
		// Login is persisted before the asynchronous initial repo sync.
		return &session, nil
	}
	if r.PDS != session.PDSUrl {
		return nil, fmt.Errorf("pds mismatch (old: %s, new: %s): please log in again", session.PDSUrl, r.PDS)
	}
	return &session, nil
}

func (state *StatefulDB) UpdateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	if state.stationStore != nil {
		return state.updateStationOAuthSession(id, session)
	}
	res := state.DB.Model(&oatproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id).Updates(session)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return nil
}

func (state *StatefulDB) updateStationOAuthSession(id string, session *oatproxy.OAuthSession) error {
	for attempt := 0; ; attempt++ {
		submitted := *session
		committing := false
		// Re-read on each attempt. This transaction contains SQL only; OAuth
		// verification and upstream exchanges are never repeated here.
		err := state.DB.Transaction(func(tx *gorm.DB) error {
			var current oatproxy.OAuthSession
			if err := tx.First(&current, "downstream_dpop_jkt = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return stationdb.ErrConflict
				}
				return err
			}
			q := tx.Model(&oatproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id)
			revision := nextOAuthSessionRevision(current.UpdatedAt)
			q = q.Session(&gorm.Session{NowFunc: func() time.Time { return revision }})
			var update any = &submitted
			switch {
			case submitted.UpdatedAt.IsZero():
				// PAR submits a partial update, not the session it just read.
				q = q.Where("downstream_par_request_uri = '' AND downstream_access_token = '' AND revoked_at IS NULL AND downstream_revoked_at IS NULL")
			case !submitted.UpdatedAt.Equal(current.UpdatedAt):
				patch, err := rebaseStationOAuthUpdate(id, &current, &submitted)
				if err != nil {
					return err
				}
				update = patch
			case submitted.DownstreamJTICache != current.DownstreamJTICache &&
				(current.RevokedAt != nil || current.DownstreamRevokedAt != nil):
				return stationdb.ErrConflict
			}
			submitted.UpdatedAt = revision
			res := q.Updates(update)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return stationdb.ErrConflict
			}
			committing = true
			return nil
		})
		if err == nil {
			*session = submitted
			return nil
		}
		if !committing || !retryStationWrite(state.DB.Statement.Context, attempt, err) {
			return err
		}
	}
}

func nextOAuthSessionRevision(previous time.Time) time.Time {
	if now := time.Now().UTC(); now.After(previous) {
		return now
	}
	return previous.Add(time.Nanosecond)
}

// A stale update can only append one verified proof or add revocation markers
// without changing the credentials or discarding any committed proofs.
func rebaseStationOAuthUpdate(id string, current, submitted *oatproxy.OAuthSession) (map[string]any, error) {
	if id == "" || submitted.DownstreamDPoPJKT != id {
		return nil, stationdb.ErrConflict
	}
	base := *submitted
	patch := make(map[string]any, 2)
	if current.RevokedAt == nil && submitted.RevokedAt != nil {
		patch["revoked_at"] = submitted.RevokedAt
		base.RevokedAt = nil
	}
	if current.DownstreamRevokedAt == nil && submitted.DownstreamRevokedAt != nil {
		patch["downstream_revoked_at"] = submitted.DownstreamRevokedAt
		base.DownstreamRevokedAt = nil
	}
	if !sameOAuthSessionState(base, current) {
		return nil, stationdb.ErrConflict
	}
	proof := len(patch) == 0
	if proof && current.Status() != oatproxy.OAuthSessionStateReady && current.Status() != oatproxy.OAuthSessionStateDownstream {
		return nil, stationdb.ErrConflict
	}
	cache, err := mergeOAuthSessionProofs(current.DownstreamJTICache, submitted.DownstreamJTICache, proof)
	if err != nil {
		return nil, err
	}
	if proof {
		patch["downstream_jti_cache"] = cache
	}
	submitted.DownstreamJTICache = cache
	return patch, nil
}

func sameOAuthSessionState(submitted oatproxy.OAuthSession, current *oatproxy.OAuthSession) bool {
	if !submitted.CreatedAt.Equal(current.CreatedAt) ||
		!sameOAuthSessionTime(submitted.UpstreamAccessTokenExp, current.UpstreamAccessTokenExp) ||
		!sameOAuthSessionTime(submitted.DownstreamPARUsedAt, current.DownstreamPARUsedAt) ||
		!sameOAuthSessionTime(submitted.RevokedAt, current.RevokedAt) ||
		!sameOAuthSessionTime(submitted.DownstreamRevokedAt, current.DownstreamRevokedAt) {
		return false
	}
	submitted.CreatedAt = current.CreatedAt
	submitted.UpstreamAccessTokenExp = current.UpstreamAccessTokenExp
	submitted.DownstreamPARUsedAt = current.DownstreamPARUsedAt
	submitted.RevokedAt = current.RevokedAt
	submitted.DownstreamRevokedAt = current.DownstreamRevokedAt
	submitted.UpdatedAt = current.UpdatedAt
	submitted.DownstreamJTICache = current.DownstreamJTICache
	// Include new comparable fields by default; noncomparable fields require a review.
	return submitted == *current
}

func sameOAuthSessionTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func mergeOAuthSessionProofs(latest, submitted string, appendProof bool) (string, error) {
	if !appendProof && submitted == latest {
		return latest, nil
	}
	var proposed []oatproxy.JTICacheEntry
	if submitted != "" {
		if err := json.Unmarshal([]byte(submitted), &proposed); err != nil || proposed == nil {
			return "", stationdb.ErrConflict
		}
	}
	now := time.Now()
	cutoff := now.Add(-time.Duration(oatproxy.EpochLength * oatproxy.ValidEpochs))
	var proof oatproxy.JTICacheEntry
	if appendProof {
		if len(proposed) == 0 {
			return "", stationdb.ErrConflict
		}
		// oatproxy.CacheJTI appends the newly verified proof after the cached history.
		proof = proposed[len(proposed)-1]
		proposed = proposed[:len(proposed)-1]
		if proof.JTI == "" || proof.Time.Before(cutoff) || proof.Time.After(now) {
			return "", stationdb.ErrConflict
		}
	}

	var current []oatproxy.JTICacheEntry
	if latest != "" {
		if err := json.Unmarshal([]byte(latest), &current); err != nil || current == nil {
			return "", stationdb.ErrConflict
		}
	}
	known := make(map[string]time.Time, len(current))
	live := current[:0]
	for _, entry := range current {
		if entry.JTI == "" || entry.Time.IsZero() || entry.JTI == proof.JTI || !known[entry.JTI].IsZero() {
			return "", stationdb.ErrConflict
		}
		known[entry.JTI] = entry.Time
		if !entry.Time.Before(cutoff) {
			live = append(live, entry)
		}
	}

	seen := make(map[string]struct{}, len(proposed))
	if appendProof {
		seen[proof.JTI] = struct{}{}
	}
	for _, entry := range proposed {
		if _, duplicate := seen[entry.JTI]; duplicate || entry.JTI == "" || entry.Time.IsZero() {
			return "", stationdb.ErrConflict
		}
		seen[entry.JTI] = struct{}{}
		if !entry.Time.Before(cutoff) && !known[entry.JTI].Equal(entry.Time) {
			return "", stationdb.ErrConflict
		}
	}
	if !appendProof {
		return latest, nil
	}
	encoded, err := json.Marshal(append(live, proof))
	return string(encoded), err
}

func (state *StatefulDB) ListOAuthSessions() ([]oatproxy.OAuthSession, error) {
	var sessions []oatproxy.OAuthSession
	if err := state.DB.Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (state *StatefulDB) GetSessionByDID(did string) (*oatproxy.OAuthSession, error) {
	var session oatproxy.OAuthSession
	if err := state.DB.Where("repo_did = ? AND revoked_at IS NULL", did).Order("updated_at DESC").First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// ErrNoSessionWithScope means the user has valid sessions, but none of them
// was granted the required scope — i.e. they declined those permissions on
// every device they're logged in on.
var ErrNoSessionWithScope = errors.New("no session with required scope")

// GetSessionByDIDWithScope returns the most recently used valid session for
// the DID that was granted every scope value in scope (space-separated).
// Sessions from before scope tracking count as full grants.
func (state *StatefulDB) GetSessionByDIDWithScope(did string, scope string) (*oatproxy.OAuthSession, error) {
	var sessions []oatproxy.OAuthSession
	if err := state.DB.Where("repo_did = ? AND revoked_at IS NULL", did).Order("updated_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	for i := range sessions {
		if sessions[i].HasScope(scope) {
			return &sessions[i], nil
		}
	}
	return nil, fmt.Errorf("%w: did=%s scope=%s", ErrNoSessionWithScope, did, scope)
}

// GetSpaceSession returns the most recently used valid session for the DID
// that was granted their personal space, or nil when there is none.
func (state *StatefulDB) GetSpaceSession(did string) (*oatproxy.OAuthSession, error) {
	var sessions []oatproxy.OAuthSession
	if err := state.DB.Where("repo_did = ? AND revoked_at IS NULL", did).Order("updated_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	for i := range sessions {
		if state.StationShards != nil {
			s := &sessions[i]
			if (s.Status() == oatproxy.OAuthSessionStateReady || s.Status() == oatproxy.OAuthSessionStateReadyUpstream) &&
				spaces.CollectionGrant(s.UpstreamScope, did, spaces.TargetCollection) {
				return s, nil
			}
			continue
		}
		if spaces.Granted(sessions[i].GrantedScope()) {
			return &sessions[i], nil
		}
	}
	return nil, nil
}
