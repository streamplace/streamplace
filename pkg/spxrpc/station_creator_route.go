package spxrpc

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/stationshard"
)

const creatorHopHeader = "X-Streamplace-Creator-Hop"
const creatorPublishDraftPath = "/xrpc/place.stream.vod.publishDraft"

type creatorRouter struct {
	slots interface {
		Owner(context.Context, string) (string, error)
		Route(context.Context, string) (string, error)
	}
	peers  map[string]*url.URL
	client *http.Client
}

type creatorRouteContextKey struct{}

type creatorSpaceOnlyContextKey struct{}

func creatorSpaceOnly(ctx context.Context) bool {
	spaceOnly, _ := ctx.Value(creatorSpaceOnlyContextKey{}).(bool)
	return spaceOnly
}

type creatorRoute struct {
	did    string
	sender string
}

func creatorMethod(path string) string {
	switch path {
	case "/xrpc/place.stream.vod.getDraft",
		"/xrpc/place.stream.vod.listDrafts",
		"/xrpc/place.stream.multistream.listTargets":
		return http.MethodGet
	case "/xrpc/place.stream.vod.createDraft",
		"/xrpc/place.stream.vod.deleteDraft",
		creatorPublishDraftPath,
		"/xrpc/place.stream.vod.updateDraft",
		"/xrpc/place.stream.multistream.createTarget",
		"/xrpc/place.stream.multistream.deleteTarget",
		"/xrpc/place.stream.multistream.putTarget":
		return http.MethodPost
	default:
		return ""
	}
}

// JWT claims are only a hint for the read-only lookup. The owner verifies the
// credential and binds the hint to the authenticated DID before routing locally.
func creatorHint(r *http.Request) string {
	scheme, raw, _ := strings.Cut(r.Header.Get(echo.HeaderAuthorization), " ")
	if scheme != "Bearer" && (scheme != "DPoP" || r.Header.Get("DPoP") == "") {
		return ""
	}
	var claims serviceAuthClaims
	if _, _, err := jwt.NewParser().ParseUnverified(raw, &claims); err != nil {
		return ""
	}
	did := claims.Subject
	if scheme == "Bearer" {
		if claims.Lxm != strings.TrimPrefix(r.URL.Path, "/xrpc/") {
			return ""
		}
		did = claims.Issuer
	}
	if _, err := syntax.ParseDID(did); err != nil {
		return ""
	}
	return did
}

func (s *Server) creatorDispatch(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		routes := s.creatorRoutes
		if routes == nil {
			return next(c)
		}
		r := c.Request()
		ctx := r.Context()
		method := creatorMethod(r.URL.Path)
		if method != "" {
			c.Response().Header().Set("Cache-Control", "private, no-store")
		}
		hops := r.Header.Values(creatorHopHeader)
		var route creatorRoute
		if len(hops) != 0 {
			peer := GetServiceAuth(ctx)
			if len(hops) != 1 || method == "" || r.Method != method || r.URL.EscapedPath() != r.URL.Path ||
				(r.TLS == nil && !s.cli.BehindHTTPSProxy) || peer == nil || peer.DID == s.cli.ServerDID() || routes.peers[peer.DID] == nil {
				return echo.NewHTTPError(http.StatusForbidden, "invalid creator hop")
			}
			if _, err := syntax.ParseDID(hops[0]); err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "invalid creator hop")
			}
			route = creatorRoute{did: hops[0], sender: peer.DID}
		} else {
			if method == "" {
				return next(c)
			}
			if r.Method != method {
				c.Response().Header().Set(echo.HeaderAllow, method)
				return echo.ErrMethodNotAllowed
			}
			if r.URL.EscapedPath() != r.URL.Path {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid creator method path")
			}
			if len(r.Header.Values(serviceAuthHeader)) != 0 {
				return echo.NewHTTPError(http.StatusForbidden, "invalid creator hop")
			}
			route.did = creatorHint(r)
			if route.did == "" {
				return next(c)
			}
		}

		owner, err := routes.slots.Owner(ctx, route.did)
		if errors.Is(err, stationshard.ErrNotFound) {
			owner = ""
		} else if err != nil {
			return creatorOwnerUnavailable()
		}
		local := s.cli.ServerDID()
		if route.sender != "" {
			if owner != local {
				return creatorOwnerChanged()
			}
		} else if owner != "" && owner != local {
			return s.forwardCreatorRequest(c, owner, route.did)
		}
		c.SetRequest(r.WithContext(context.WithValue(ctx, creatorRouteContextKey{}, route)))
		return next(c)
	}
}

func (s *Server) creatorAuthorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		routes := s.creatorRoutes
		if routes == nil || creatorMethod(r.URL.Path) == "" {
			return next(c)
		}
		ctx := r.Context()
		caller := GetCaller(ctx)
		if caller == nil || (r.URL.Path == creatorPublishDraftPath && caller.OAuth == nil) {
			return echo.NewHTTPError(http.StatusUnauthorized, "creator authentication required")
		}
		route, _ := ctx.Value(creatorRouteContextKey{}).(creatorRoute)
		if route.did == "" || route.did != caller.DID {
			return echo.NewHTTPError(http.StatusForbidden, "creator DID does not match authenticated caller")
		}
		if caller.OAuth != nil {
			allowed, spaceOnly := creatorOAuthAccess(caller.OAuth, r.URL.Path)
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient creator scope")
			}
			if spaceOnly {
				ctx = context.WithValue(ctx, creatorSpaceOnlyContextKey{}, true)
				c.SetRequest(r.WithContext(ctx))
			}
		}

		// Authentication may have consumed the proof. Never forward from here.
		owner, err := routes.slots.Route(ctx, caller.DID)
		if err != nil || owner == "" {
			return creatorOwnerUnavailable()
		}
		if owner != s.cli.ServerDID() {
			return creatorOwnerChanged()
		}
		return next(c)
	}
}

func creatorOwnerUnavailable() error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "creator owner unavailable")
}

func creatorOwnerChanged() error {
	return echo.NewHTTPError(http.StatusConflict, "creator owner changed")
}

func newCreatorPeerClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	// A fresh HTTP/1 connection prevents transport retries on a used connection
	// or after an HTTP/2 GOAWAY. A forwarded DPoP proof is single-use.
	transport.DisableKeepAlives = true
	transport.Protocols = new(http.Protocols)
	transport.Protocols.SetHTTP1(true)
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}
}

func (s *Server) forwardCreatorRequest(c echo.Context, owner, did string) error {
	routes := s.creatorRoutes
	origin := routes.peers[owner]
	if origin == nil || origin.Scheme != "https" || routes.client == nil || s.cli.ServiceAuthKey == nil {
		return creatorOwnerUnavailable()
	}
	token, err := CreateServiceToken(s.cli.ServiceAuthKey, s.cli.ServerDID())
	if err != nil {
		return creatorOwnerUnavailable()
	}
	r := c.Request()
	target := url.URL{Scheme: "https", Host: origin.Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		return creatorOwnerUnavailable()
	}
	req.ContentLength = r.ContentLength
	req.GetBody = nil
	req.Close = true
	for _, key := range [...]string{echo.HeaderAuthorization, echo.HeaderContentType, "DPoP"} {
		if v := r.Header.Get(key); v != "" {
			req.Header.Set(key, v)
		}
	}
	req.Header.Set(creatorHopHeader, did)
	req.Header.Set(serviceAuthHeader, token)

	client := *routes.client
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return creatorOwnerUnavailable()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		return creatorOwnerUnavailable()
	}
	for _, key := range [...]string{
		"DPoP-Nonce", "WWW-Authenticate",
		"Access-Control-Allow-Origin", "Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods", "Access-Control-Expose-Headers",
	} {
		if values := resp.Header.Values(key); len(values) != 0 {
			c.Response().Header()[http.CanonicalHeaderKey(key)] = values
		}
	}
	return c.Stream(resp.StatusCode, resp.Header.Get(echo.HeaderContentType), resp.Body)
}
