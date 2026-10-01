package spxrpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

const serviceAuthLeeway = 5 * time.Second

var serviceAuthAlgs = []string{"ES256", "ES256K"}

type Caller struct {
	DID   string
	OAuth *oatproxy.OAuthSession
}

type pdsAuthContextKeyType struct{}

var pdsAuthContextKey = pdsAuthContextKeyType{}

func GetCaller(ctx context.Context) *Caller {
	if session, _ := oatproxy.GetOAuthSession(ctx); session != nil {
		return &Caller{DID: session.DID, OAuth: session}
	}
	if did, ok := ctx.Value(pdsAuthContextKey).(string); ok && did != "" {
		return &Caller{DID: did}
	}
	return nil
}

func (s *Server) serviceAudiences() []string {
	dids := []string{s.cli.ServerDID(), s.cli.BroadcasterDID()}
	auds := make([]string, 0, len(dids)*2)
	for _, did := range dids {
		auds = append(auds, did, did+"#"+atproto.StreamplaceServiceID)
	}
	return auds
}

func (s *Server) useAuthMiddleware(e *echo.Echo) {
	e.Use(s.ServiceAuthMiddleware())
	e.Use(s.op.OAuthMiddleware)
	e.Use(s.PDSAuthMiddleware())
}

func (s *Server) PDSAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			raw, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || s.identityDirectory == nil {
				return next(c)
			}
			method, ok := strings.CutPrefix(c.Request().URL.Path, "/xrpc/")
			if !ok {
				return next(c)
			}
			if _, err := syntax.ParseNSID(method); err != nil {
				return next(c)
			}

			ctx := c.Request().Context()
			token, err := parseServiceToken(raw)
			if err == nil {
				err = token.checkClaims(time.Now(), method, s.serviceAudiences())
			}
			if err == nil {
				err = verifyServiceToken(ctx, s.identityDirectory(), token)
			}
			if err != nil {
				log.Debug(ctx, "rejected PDS service auth token", "method", method, "error", err)
				return next(c)
			}

			ctx = context.WithValue(ctx, pdsAuthContextKey, token.issuer.String())
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

type serviceToken struct {
	signingInput []byte
	signature    []byte
	issuer       syntax.DID
	audience     []string
	lxm          string
	expires      time.Time
	issuedAt     time.Time
}

func parseServiceToken(raw string) (*serviceToken, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}

	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if !slices.Contains(serviceAuthAlgs, header.Alg) {
		return nil, fmt.Errorf("unsupported alg %q", header.Alg)
	}
	if len(signature) != 64 {
		return nil, errors.New("signature has the wrong length")
	}

	var claims struct {
		Iss string          `json:"iss"`
		Aud json.RawMessage `json:"aud"`
		Exp *int64          `json:"exp"`
		Iat *int64          `json:"iat"`
		Lxm string          `json:"lxm"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	issuer, err := syntax.ParseDID(claims.Iss)
	if err != nil {
		return nil, fmt.Errorf("iss: %w", err)
	}
	if claims.Exp == nil || claims.Iat == nil {
		return nil, errors.New("exp and iat are required")
	}
	audience, err := parseAudience(claims.Aud)
	if err != nil {
		return nil, err
	}

	return &serviceToken{
		signingInput: []byte(parts[0] + "." + parts[1]),
		signature:    signature,
		issuer:       issuer,
		audience:     audience,
		lxm:          claims.Lxm,
		expires:      time.Unix(*claims.Exp, 0),
		issuedAt:     time.Unix(*claims.Iat, 0),
	}, nil
}

func parseAudience(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, errors.New("aud must be a string or a list of strings")
	}
	return many, nil
}

func (t *serviceToken) checkClaims(now time.Time, method string, audiences []string) error {
	if !slices.ContainsFunc(t.audience, func(aud string) bool { return slices.Contains(audiences, aud) }) {
		return fmt.Errorf("aud %v is not this service", t.audience)
	}
	if t.lxm != method {
		return fmt.Errorf("lxm %q does not match %q", t.lxm, method)
	}
	if now.After(t.expires.Add(serviceAuthLeeway)) {
		return errors.New("token expired")
	}
	if t.issuedAt.After(now.Add(serviceAuthLeeway)) {
		return errors.New("token issued in the future")
	}
	return nil
}

func verifyServiceToken(ctx context.Context, dir identity.Directory, t *serviceToken) error {
	err := verifyServiceTokenSignature(ctx, dir, t)
	if err == nil {
		return nil
	}
	if purgeErr := dir.Purge(ctx, t.issuer.AtIdentifier()); purgeErr != nil {
		log.Warn(ctx, "failed to purge identity before retrying service auth", "did", t.issuer, "error", purgeErr)
	}
	return verifyServiceTokenSignature(ctx, dir, t)
}

func verifyServiceTokenSignature(ctx context.Context, dir identity.Directory, t *serviceToken) error {
	ident, err := dir.LookupDID(ctx, t.issuer)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", t.issuer, err)
	}
	key, err := ident.PublicKey()
	if err != nil {
		return fmt.Errorf("signing key for %s: %w", t.issuer, err)
	}
	return key.HashAndVerifyLenient(t.signingInput, t.signature)
}
