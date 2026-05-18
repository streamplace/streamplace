package statedb

import (
	"errors"
	"fmt"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
)

func (state *StatefulDB) CreateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	return state.DB.Create(session).Error
}

func (state *StatefulDB) LoadOAuthSession(id string) (*oatproxy.OAuthSession, error) {
	var session oatproxy.OAuthSession
	if err := state.DB.Where("downstream_dpop_jkt = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if session.Status() != oatproxy.OAuthSessionStateReady || session.Status() != oatproxy.OAuthSessionStateReadyUpstream {
		return &session, nil
	}
	r, err := state.model.GetRepo(session.DID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	if r == nil {
		return nil, fmt.Errorf("repo not found even though we have a valid session!? repodid=%s", session.DID)
	}
	if r.PDS != session.PDSUrl {
		return nil, fmt.Errorf("pds mismatch (old: %s, new: %s): please log in again", session.PDSUrl, r.PDS)
	}
	return &session, nil
}

func (state *StatefulDB) UpdateOAuthSession(id string, session *oatproxy.OAuthSession) error {
	res := state.DB.Model(&oatproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id).Updates(session)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return nil
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
