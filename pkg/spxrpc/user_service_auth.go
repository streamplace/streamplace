package spxrpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
)

// UserServiceAuth is a caller proven by an atproto service-auth token: a JWT
// its PDS signed with the account's atproto key (com.atproto.server
// .getServiceAuth), addressed to this node and to the one method called.
//
// It is how an app that holds its own session at the user's PDS (and so
// none the node can attribute) acts for that user here without a second
// login: the token proves who the caller is, and the node writes to the
// user's repo with the session it stored when the user once signed in to it
// with OAuth. Handlers opt in to it one by one; everything else still sees
// an anonymous caller.
type UserServiceAuth struct {
	DID string
}

type userServiceAuthContextKeyType struct{}

var userServiceAuthContextKey = userServiceAuthContextKeyType{}

// GetUserServiceAuth returns the caller proven by a user service-auth token,
// or nil when the request carried none (or an invalid one).
func GetUserServiceAuth(ctx context.Context) *UserServiceAuth {
	v, _ := ctx.Value(userServiceAuthContextKey).(*UserServiceAuth)
	return v
}

// userServiceAuthAudiences are the DIDs a token may name as its audience:
// this server's own and, when it differs, the broadcaster it serves.
func (s *Server) userServiceAuthAudiences() []string {
	auds := []string{s.cli.ServerDID()}
	if b := s.cli.BroadcasterDID(); b != auds[0] {
		auds = append(auds, b)
	}
	return auds
}

// identityDirectory resolves the DID documents service-auth tokens are
// checked against.
func (s *Server) identityDirectory() identity.Directory {
	s.userAuthDirOnce.Do(func() {
		if s.userAuthDir != nil {
			return
		}
		dir := identity.NewCacheDirectory(atproto.CustomDirectory(s.cli.PLCURL), 50_000, time.Hour, time.Minute*2, time.Minute*5)
		s.userAuthDir = &dir
	})
	return s.userAuthDir
}

// UserServiceAuthMiddleware attributes a request carrying
// "Authorization: Bearer <service-auth JWT>" to the token's issuer when the
// token is addressed to this node and to the XRPC method being called. A
// missing or invalid token leaves the request anonymous, as before: other
// bearer tokens (a PDS access token sent along by mistake, say) are not this
// middleware's business.
func (s *Server) UserServiceAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			hdr := c.Request().Header.Get("Authorization")
			token, ok := strings.CutPrefix(hdr, "Bearer ")
			if !ok || token == "" {
				return next(c)
			}
			method, ok := xrpcMethod(c.Request().URL.Path)
			if !ok {
				return next(c)
			}
			ctx := c.Request().Context()
			did, err := s.validateUserServiceAuth(ctx, token, method)
			if err != nil {
				log.Debug(ctx, "ignoring bearer token that is not a valid service-auth token for this node", "method", method.String(), "error", err)
				return next(c)
			}
			ctx = context.WithValue(ctx, userServiceAuthContextKey, &UserServiceAuth{DID: did.String()})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func (s *Server) validateUserServiceAuth(ctx context.Context, token string, method syntax.NSID) (syntax.DID, error) {
	var lastErr error
	for _, aud := range s.userServiceAuthAudiences() {
		v := auth.ServiceAuthValidator{Audience: aud, Dir: s.identityDirectory()}
		did, err := v.Validate(ctx, token, &method)
		if err == nil {
			return did, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// xrpcMethod is the NSID of an /xrpc/<method> path.
func xrpcMethod(path string) (syntax.NSID, bool) {
	rest, ok := strings.CutPrefix(path, "/xrpc/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	nsid, err := syntax.ParseNSID(rest)
	if err != nil {
		return "", false
	}
	return nsid, true
}

// storedStreamerClient is the client the node writes to did's repo with on
// its own: its stored OAuth session for the account (or development
// credentials). Without one the caller has to sign in to the node with OAuth
// once, which the error says in a form clients can match on.
func (s *Server) storedStreamerClient(ctx context.Context, did string) (statedb.UserClient, error) {
	if !s.statefulDB.HasUserSession(did) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "NodeSessionRequired: sign in to this node with OAuth once so it can act for this account")
	}
	client, err := s.statefulDB.UserXrpcClient(ctx, did)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting streamer client", err)
	}
	return client, nil
}

// canPostAs reports whether the node may create a Bluesky post for did:
// the caller's own OAuth session decides when there is one, otherwise any
// stored session that was granted post creation (or development
// credentials, which carry every scope).
func (s *Server) canPostAs(ctx context.Context, did string, session *oatproxy.OAuthSession) bool {
	if session != nil {
		return session.HasScope(atproto.ScopeBskyPostCreate)
	}
	if s.cli.DevAccountCreds[did] != "" {
		return true
	}
	if _, err := s.statefulDB.GetSessionByDIDWithScope(did, atproto.ScopeBskyPostCreate); err != nil {
		log.Debug(ctx, "no stored session with post permission", "did", did, "err", err)
		return false
	}
	return true
}
