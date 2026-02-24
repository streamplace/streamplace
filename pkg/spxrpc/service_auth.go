package spxrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"stream.place/streamplace/pkg/log"
)

const serviceAuthHeader = "X-Streamplace-Service-Auth"
const serviceTokenLifetime = 5 * time.Minute

// ServiceIdentity represents an authenticated peer node in the same station.
type ServiceIdentity struct {
	DID string
}

type serviceAuthContextKeyType struct{}

var serviceAuthContextKey = serviceAuthContextKeyType{}

// GetServiceAuth returns the authenticated service identity from the context,
// or nil if the request did not come from an authenticated peer node.
func GetServiceAuth(ctx context.Context) *ServiceIdentity {
	v := ctx.Value(serviceAuthContextKey)
	if v == nil {
		return nil
	}
	identity, ok := v.(*ServiceIdentity)
	if !ok {
		return nil
	}
	return identity
}

// ServiceAuthMiddleware checks for a service-to-service JWT in the
// X-Streamplace-Service-Auth header. If present and valid, it populates
// the context with the caller's ServiceIdentity. If absent or invalid,
// the request passes through unchanged for the normal OAuth flow.
func (s *Server) ServiceAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr := c.Request().Header.Get(serviceAuthHeader)
			if tokenStr == "" {
				return next(c)
			}

			ctx := c.Request().Context()
			key := s.cli.ServiceAuthKey
			if key == nil {
				log.Warn(ctx, "service auth token present but no service auth key configured")
				return next(c)
			}

			token, err := jwt.Parse([]byte(tokenStr), jwt.WithKey(jwa.HS256, key), jwt.WithValidate(true))
			if err != nil {
				log.Warn(ctx, "invalid service auth token", "error", err)
				return next(c)
			}

			issuer := token.Issuer()
			if issuer == "" {
				log.Warn(ctx, "service auth token missing issuer claim")
				return next(c)
			}

			identity := &ServiceIdentity{DID: issuer}
			ctx = context.WithValue(ctx, serviceAuthContextKey, identity)
			c.SetRequest(c.Request().WithContext(ctx))
			log.Warn(ctx, "authenticated service request", "service_did", issuer)
			return next(c)
		}
	}
}

// CreateServiceToken generates a signed JWT for authenticating this node
// to another node in the same station.
func CreateServiceToken(key jwk.Key, serverDID string) (string, error) {
	now := time.Now()
	token, err := jwt.NewBuilder().
		Issuer(serverDID).
		IssuedAt(now).
		Expiration(now.Add(serviceTokenLifetime)).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build service token: %w", err)
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256, key))
	if err != nil {
		return "", fmt.Errorf("failed to sign service token: %w", err)
	}

	return string(signed), nil
}
