package statedb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"gorm.io/gorm"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/log"
)

// UserClient is an XRPC client acting as one user: the shape both the OAuth
// proxy's client and a plain bearer client share.
type UserClient interface {
	Do(ctx context.Context, kind string, inpenc string, method string, params map[string]any, bodyobj any, out any) error
}

// passwordSessionTTL is how long a session made from an app password is
// used before a fresh one is created; access tokens last longer, but a
// re-login an hour is cheap and needs no refresh bookkeeping.
const passwordSessionTTL = time.Hour

type passwordSession struct {
	client  *xrpc.Client
	created time.Time
}

// UserXrpcClient is the node's client for writing to a user's repo. It comes
// from the user's stored OAuth session, or, for an account the operator
// gave the node credentials for (--account-credentials / --dev-account-creds),
// from a session created with that app password, cached for an hour and
// made again on demand. The credential path is for a network whose OAuth is
// broken or absent: the streamer never signs in here, yet the node still
// starts, updates and ends their livestream records, publishes their VODs
// and acts for their moderators.
func (state *StatefulDB) UserXrpcClient(ctx context.Context, did string) (UserClient, error) {
	if c, err, ok := state.passwordClient(ctx, did); ok {
		return c, err
	}
	session, err := state.GetSessionByDID(did)
	if err != nil {
		return nil, fmt.Errorf("get session for %s: %w", did, err)
	}
	if session == nil {
		return nil, fmt.Errorf("no session for %s", did)
	}
	if state.OATProxy == nil {
		return nil, fmt.Errorf("no OAuth proxy configured")
	}
	session, err = state.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		return nil, fmt.Errorf("refresh session for %s: %w", did, err)
	}
	client, err := state.OATProxy.GetXrpcClient(session)
	if err != nil {
		return nil, fmt.Errorf("get xrpc client for %s: %w", did, err)
	}
	return client, nil
}

// HasUserSession reports whether UserXrpcClient can act for did: the node
// holds credentials for the account, or a stored OAuth session.
func (state *StatefulDB) HasUserSession(did string) bool {
	if state.CLI != nil && state.CLI.DevAccountCreds[did] != "" {
		return true
	}
	session, err := state.GetSessionByDID(did)
	if err != nil || session == nil {
		return false
	}
	return true
}

// passwordClient is the cached app-password session for did, when the node
// has credentials for it; ok is false when it has none.
func (state *StatefulDB) passwordClient(ctx context.Context, did string) (UserClient, error, bool) {
	if state.CLI == nil {
		return nil, nil, false
	}
	password, has := state.CLI.DevAccountCreds[did]
	if !has || password == "" {
		return nil, nil, false
	}
	state.passwordMu.Lock()
	defer state.passwordMu.Unlock()
	if state.passwordSessions == nil {
		state.passwordSessions = map[string]*passwordSession{}
	}
	if ps := state.passwordSessions[did]; ps != nil && time.Since(ps.created) < passwordSessionTTL {
		return ps.client, nil, true
	}
	if state.model == nil {
		return nil, fmt.Errorf("no repo index to find the PDS of %s", did), true
	}
	repo, err := state.model.GetRepoByHandleOrDID(did)
	if err != nil {
		return nil, fmt.Errorf("look up %s: %w", did, err), true
	}
	if repo == nil || repo.PDS == "" {
		return nil, fmt.Errorf("%s is not indexed on this node, so its PDS is unknown", did), true
	}
	anon := &xrpc.Client{Host: repo.PDS, Client: &aqhttp.Client}
	out, err := comatproto.ServerCreateSession(ctx, anon, &comatproto.ServerCreateSession_Input{
		Identifier: repo.DID,
		Password:   password,
	})
	if err != nil {
		return nil, fmt.Errorf("create session for %s at %s: %w", did, repo.PDS, err), true
	}
	log.Log(ctx, "created a session from the node's credentials for an account", "did", repo.DID, "handle", repo.Handle, "pds", repo.PDS)
	client := &xrpc.Client{
		Host:   repo.PDS,
		Client: &aqhttp.Client,
		Auth: &xrpc.AuthInfo{
			Did:        repo.DID,
			Handle:     repo.Handle,
			AccessJwt:  out.AccessJwt,
			RefreshJwt: out.RefreshJwt,
		},
	}
	state.passwordSessions[did] = &passwordSession{client: client, created: time.Now()}
	return client, nil, true
}

// ForgetPasswordSession drops the cached session for did so the next use
// logs in again: for a call that failed with an expired token.
func (state *StatefulDB) ForgetPasswordSession(did string) {
	state.passwordMu.Lock()
	defer state.passwordMu.Unlock()
	delete(state.passwordSessions, did)
}

// IsNoSession reports whether err means the node cannot act for the user
// at all (no credentials and no stored session), as opposed to a failure
// while acting.
func IsNoSession(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) || (err != nil && strings.Contains(err.Error(), "no session for"))
}

var _ = sync.Mutex{}
