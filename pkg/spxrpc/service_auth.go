package spxrpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"stream.place/streamplace/pkg/aqhttp"
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
			key := s.ServiceAuthKey
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
			log.Log(ctx, "got authenticated service request", "service_did", issuer)
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

// DIDWebToHost extracts the hostname from a did:web DID.
func DIDWebToHost(did string) string {
	return strings.TrimPrefix(did, "did:web:")
}

// ProxyServiceRequest proxies an XRPC request to a peer node, authenticating
// with a service token. Returns the response body bytes or an echo.HTTPError.
func (s *Server) ProxyServiceRequest(ctx context.Context, targetDID, httpMethod, xrpcMethod string, query url.Values, body io.Reader, contentType string) ([]byte, error) {
	token, err := CreateServiceToken(s.ServiceAuthKey, s.cli.ServerDID())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error creating service token: "+err.Error())
	}

	u := url.URL{
		Scheme:   "https",
		Host:     DIDWebToHost(targetDID),
		Path:     "/xrpc/" + xrpcMethod,
		RawQuery: query.Encode(),
	}

	log.Log(ctx, "proxying service request", "target", targetDID, "method", xrpcMethod)

	req, err := http.NewRequestWithContext(ctx, httpMethod, u.String(), body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to construct proxy request: "+err.Error())
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set(serviceAuthHeader, token)

	resp, err := aqhttp.Client.Do(req)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadGateway, "error proxying to peer: "+err.Error())
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "error reading peer response: "+err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, echo.NewHTTPError(resp.StatusCode, "peer error: "+string(data))
	}
	return data, nil
}
