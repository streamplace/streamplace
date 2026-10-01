package spxrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

const serviceAuthLeeway = 5 * time.Second

const identityRefreshInterval = time.Minute

var serviceAuthAlgs = []string{jwt.SigningMethodES256.Alg(), signingMethodES256K.Alg()}

var signingMethodES256K = es256kSigningMethod{}

func init() {
	jwt.RegisterSigningMethod(signingMethodES256K.Alg(), func() jwt.SigningMethod { return signingMethodES256K })
}

type es256kSigningMethod struct{}

func (es256kSigningMethod) Alg() string { return "ES256K" }

func (es256kSigningMethod) Verify(signingString string, sig []byte, key any) error {
	pub, ok := key.(*atcrypto.PublicKeyK256)
	if !ok {
		return jwt.ErrInvalidKeyType
	}
	if len(sig) != 64 {
		return jwt.ErrTokenSignatureInvalid
	}
	return pub.HashAndVerifyLenient([]byte(signingString), sig)
}

func (es256kSigningMethod) Sign(string, any) ([]byte, error) {
	return nil, errors.New("ES256K signing is not supported")
}

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
			issuer, err := s.verifyServiceToken(ctx, raw, method)
			if err != nil {
				log.Debug(ctx, "rejected PDS service auth token", "method", method, "error", err)
				return next(c)
			}

			ctx = context.WithValue(ctx, pdsAuthContextKey, issuer.String())
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

type serviceAuthClaims struct {
	jwt.RegisteredClaims
	Lxm string `json:"lxm"`
}

func newIdentityRefreshCache() *cache.Cache {
	return cache.New(identityRefreshInterval, 2*identityRefreshInterval)
}

func (s *Server) verifyServiceToken(ctx context.Context, raw string, method string) (syntax.DID, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods(serviceAuthAlgs),
		jwt.WithAudience(s.serviceAudiences()...),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(serviceAuthLeeway),
	)
	dir := s.identityDirectory()
	claims := &serviceAuthClaims{}
	_, err := parser.ParseWithClaims(raw, claims, issuerKey(ctx, dir))
	if errors.Is(err, jwt.ErrTokenSignatureInvalid) && s.identityRefreshes.Add(claims.Issuer, struct{}{}, cache.DefaultExpiration) == nil {
		issuer := syntax.DID(claims.Issuer)
		if purgeErr := dir.Purge(ctx, issuer.AtIdentifier()); purgeErr != nil {
			log.Warn(ctx, "failed to purge identity before retrying service auth", "did", issuer, "error", purgeErr)
		}
		claims = &serviceAuthClaims{}
		_, err = parser.ParseWithClaims(raw, claims, issuerKey(ctx, dir))
	}
	if err != nil {
		return "", err
	}
	if claims.IssuedAt == nil {
		return "", errors.New("iat is required")
	}
	if claims.Lxm != method {
		return "", fmt.Errorf("lxm %q does not match %q", claims.Lxm, method)
	}
	return syntax.DID(claims.Issuer), nil
}

func issuerKey(ctx context.Context, dir identity.Directory) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		iss, err := token.Claims.GetIssuer()
		if err != nil {
			return nil, err
		}
		did, err := syntax.ParseDID(iss)
		if err != nil {
			return nil, fmt.Errorf("iss: %w", err)
		}
		ident, err := dir.LookupDID(ctx, did)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", did, err)
		}
		key, err := ident.PublicKey()
		if err != nil {
			return nil, fmt.Errorf("signing key for %s: %w", did, err)
		}
		if p256, ok := key.(*atcrypto.PublicKeyP256); ok {
			return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), p256.UncompressedBytes())
		}
		return key, nil
	}
}
