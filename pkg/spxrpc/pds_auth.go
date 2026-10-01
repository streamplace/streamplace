package spxrpc

import (
	"context"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/atproto/auth"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

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

func (s *Server) PDSAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || s.identityDirectory == nil {
				return next(c)
			}
			method, ok := strings.CutPrefix(c.Request().URL.Path, "/xrpc/")
			if !ok {
				return next(c)
			}
			nsid, err := syntax.ParseNSID(method)
			if err != nil {
				return next(c)
			}

			ctx := c.Request().Context()
			unverified, err := jwt.ParseInsecure([]byte(token))
			if err != nil {
				return next(c)
			}
			auds := s.serviceAudiences()
			i := slices.IndexFunc(unverified.Audience(), func(aud string) bool { return slices.Contains(auds, aud) })
			if i < 0 {
				return next(c)
			}

			validator := auth.ServiceAuthValidator{
				Audience: unverified.Audience()[i],
				Dir:      s.identityDirectory(),
			}
			did, err := validator.Validate(ctx, token, &nsid)
			if err != nil {
				log.Debug(ctx, "rejected PDS service auth token", "method", method, "error", err)
				return next(c)
			}

			ctx = context.WithValue(ctx, pdsAuthContextKey, did.String())
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
