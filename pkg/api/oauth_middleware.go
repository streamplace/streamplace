package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"stream.place/streamplace/pkg/atproto"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/model"
)

func (a *StreamplaceAPI) OAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session, err := getOAuthSession(r, a.Model)
		if err != nil {
			apierrors.WriteHTTPUnauthorized(w, "could not get oauth session", err)
			return
		}
		if session == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx = context.WithValue(ctx, atproto.OAuthContextKey, session)
		ctx = context.WithValue(ctx, atproto.ModelContextKey, a.Model)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getOAuthSession(r *http.Request, mod model.Model) (*model.OAuthSession, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}
	if !strings.HasPrefix(authHeader, "DPoP ") {
		return nil, fmt.Errorf("invalid authorization header (must start with DPoP)")
	}
	token := strings.TrimPrefix(authHeader, "DPoP ")
	session, err := mod.GetOAuthSessionByDownstreamAccessToken(token)
	if err != nil {
		return nil, fmt.Errorf("could not get oauth session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("oauth session not found")
	}
	return session, nil
}
