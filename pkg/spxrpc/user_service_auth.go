package spxrpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
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
// this server's own and, when it differs, the broadcaster it serves. A host
// with a port is also accepted in did:web's own spelling of it
// ("did:web:localhost%3A8080"), which is the form a PDS will sign for.
func (s *Server) userServiceAuthAudiences() []string {
	var auds []string
	add := func(did string) {
		for _, a := range auds {
			if a == did {
				return
			}
		}
		auds = append(auds, did)
	}
	for _, did := range []string{s.cli.ServerDID(), s.cli.BroadcasterDID()} {
		add(did)
		if host, ok := strings.CutPrefix(did, "did:web:"); ok {
			add("did:web:" + strings.ReplaceAll(host, ":", "%3A"))
		}
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

// serviceAuthLeeway is the clock skew allowed on a token's exp and iat.
const serviceAuthLeeway = 5 * time.Second

var errServiceAuthSignature = errors.New("service-auth token signature does not verify")

// validateUserServiceAuth checks an atproto service-auth JWT and returns its
// issuer: signed by the issuer's atproto key (ES256 or ES256K, low-S not
// required), unexpired, addressed to this node, bound to method.
//
// Verified here with atcrypto rather than indigo's atproto/auth validator:
// importing that package registers its own "ES256" and "ES256K" with
// golang-jwt's process-wide signing-method table, which only accept atcrypto
// keys, and oatproxy's DPoP checks (golang-jwt with an *ecdsa.PublicKey) then
// fail for every OAuth request with "key is of invalid type".
func (s *Server) validateUserServiceAuth(ctx context.Context, token string, method syntax.NSID) (syntax.DID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return "", fmt.Errorf("header: %w", err)
	}
	if header.Alg != "ES256" && header.Alg != "ES256K" {
		return "", fmt.Errorf("unsupported alg %q", header.Alg)
	}
	var claims struct {
		Iss string          `json:"iss"`
		Aud json.RawMessage `json:"aud"`
		Exp *float64        `json:"exp"`
		Iat *float64        `json:"iat"`
		Lxm string          `json:"lxm"`
	}
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return "", fmt.Errorf("claims: %w", err)
	}
	now := time.Now()
	if claims.Exp == nil || time.Unix(int64(*claims.Exp), 0).Add(serviceAuthLeeway).Before(now) {
		return "", errors.New("token is expired or has no exp")
	}
	if claims.Iat != nil && time.Unix(int64(*claims.Iat), 0).Add(-serviceAuthLeeway).After(now) {
		return "", errors.New("token is issued in the future")
	}
	if claims.Lxm != method.String() {
		return "", fmt.Errorf("token is for %q, not %q", claims.Lxm, method.String())
	}
	if !audienceMatches(claims.Aud, s.userServiceAuthAudiences()) {
		return "", errors.New("token is addressed to another service")
	}
	did, err := syntax.ParseDID(claims.Iss)
	if err != nil {
		return "", fmt.Errorf("iss: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return "", errors.New("malformed signature")
	}
	signed := []byte(parts[0] + "." + parts[1])

	err = s.verifyServiceAuthSignature(ctx, did, header.Alg, signed, sig)
	if errors.Is(err, errServiceAuthSignature) {
		// The account may have rotated its key since we cached its DID
		// document: look it up afresh once.
		if perr := s.identityDirectory().Purge(ctx, did.AtIdentifier()); perr != nil {
			log.Debug(ctx, "could not purge cached identity", "did", did, "err", perr)
		}
		err = s.verifyServiceAuthSignature(ctx, did, header.Alg, signed, sig)
	}
	if err != nil {
		return "", err
	}
	return did, nil
}

func (s *Server) verifyServiceAuthSignature(ctx context.Context, did syntax.DID, alg string, signed, sig []byte) error {
	ident, err := s.identityDirectory().LookupDID(ctx, did)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", did, err)
	}
	pub, err := ident.PublicKey()
	if err != nil {
		return fmt.Errorf("atproto key of %s: %w", did, err)
	}
	switch pub.(type) {
	case *atcrypto.PublicKeyK256:
		if alg != "ES256K" {
			return fmt.Errorf("alg %s does not match a K-256 key", alg)
		}
	case *atcrypto.PublicKeyP256:
		if alg != "ES256" {
			return fmt.Errorf("alg %s does not match a P-256 key", alg)
		}
	default:
		return fmt.Errorf("unsupported key type %T", pub)
	}
	if err := pub.HashAndVerifyLenient(signed, sig); err != nil {
		return fmt.Errorf("%w: %w", errServiceAuthSignature, err)
	}
	return nil
}

func decodeJWTPart(part string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// audienceMatches reports whether a JWT aud claim (a string, or an array of
// them) names one of auds.
func audienceMatches(raw json.RawMessage, auds []string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return slices.Contains(auds, one)
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, a := range many {
			if slices.Contains(auds, a) {
				return true
			}
		}
	}
	return false
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
		log.Error(ctx, "could not act for the streamer with the node's session", "did", did, "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error getting streamer client: "+err.Error())
	}
	return client, nil
}

// postClient is the client to create the go-live post for did with, or nil
// when the node may not post for them. The caller's own OAuth session
// decides when there is one (and posts with its client); otherwise the post
// is made with a stored session that was granted post creation, which need
// not be the newest session the livestream record is written with.
func (s *Server) postClient(ctx context.Context, did string, session *oatproxy.OAuthSession, sessionClient statedb.UserClient) statedb.UserClient {
	if session != nil {
		if session.HasScope(atproto.ScopeBskyPostCreate) {
			return sessionClient
		}
		return nil
	}
	if s.statefulDB == nil {
		return nil
	}
	client, err := s.statefulDB.UserXrpcClientWithScope(ctx, did, atproto.ScopeBskyPostCreate)
	if err != nil {
		log.Debug(ctx, "no stored session with post permission", "did", did, "err", err)
		return nil
	}
	return client
}
