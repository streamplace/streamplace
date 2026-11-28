package spxrpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/stationshard"
)

const (
	stationActiveTargetsMethod = "_station.activeMultistreamTargets"
	stationActiveTargetsPath   = "/xrpc/" + stationActiveTargetsMethod
	stationActiveTargetsAuth   = "X-Streamplace-Station-Targets-Auth"
	stationActiveTargetsTTL    = time.Minute
	stationActiveTargetsLimit  = 128 << 10
)

var (
	errStationActiveTargetsUnavailable = errors.New("station active multistream targets unavailable")
	stationActiveTargetsPeerClient     = newCreatorPeerClient()
)

type stationActiveTargetsOutput struct {
	Targets []placestream.MultistreamDefs_TargetView `json:"targets"`
}

// Derive a route-specific key so this credential cannot authenticate other service requests.
func stationActiveTargetsKey(cli *config.CLI) []byte {
	var secret []byte
	if cli.ServiceAuthKey == nil || cli.ServiceAuthKey.Raw(&secret) != nil || len(secret) < sha256.Size {
		return nil
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(stationActiveTargetsMethod))
	return mac.Sum(nil)
}

// FetchStationActiveMultistreamTargets is for a trusted local stream worker's DID.
// A failed owner read never switches to a different store or peer.
func FetchStationActiveMultistreamTargets(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB, did string) ([]placestream.MultistreamDefs_TargetView, error) {
	if state.StationShards == nil {
		return nil, errStationActiveTargetsUnavailable
	}
	return fetchStationActiveMultistreamTargets(ctx, cli, state, state.StationShards, stationActiveTargetsPeerClient, did)
}

func fetchStationActiveMultistreamTargets(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB, slots interface {
	Route(context.Context, string) (string, error)
}, peerClient *http.Client, did string) ([]placestream.MultistreamDefs_TargetView, error) {
	if _, err := syntax.ParseDID(did); err != nil {
		return nil, errStationActiveTargetsUnavailable
	}
	owner, err := slots.Route(ctx, did)
	if err != nil || owner == "" {
		return nil, errStationActiveTargetsUnavailable
	}
	node := cli.ServerDID()
	if owner == node {
		return readStationActiveMultistreamTargets(ctx, state, did)
	}
	peers, err := cli.StationPeerOrigins()
	origin := peers[owner]
	if err != nil || origin == nil {
		return nil, errStationActiveTargetsUnavailable
	}
	key := stationActiveTargetsKey(cli)
	if key == nil {
		return nil, errStationActiveTargetsUnavailable
	}
	claims := serviceAuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: node, Subject: did, Audience: jwt.ClaimStrings{owner},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(stationActiveTargetsTTL)),
		},
		Lxm: stationActiveTargetsMethod,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		return nil, errStationActiveTargetsUnavailable
	}
	endpoint := url.URL{Scheme: origin.Scheme, Host: origin.Host, Path: stationActiveTargetsPath}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, errStationActiveTargetsUnavailable
	}
	req.Header.Set(stationActiveTargetsAuth, token)
	client := *peerClient
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, errStationActiveTargetsUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errStationActiveTargetsUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, stationActiveTargetsLimit+1))
	if err != nil || len(body) > stationActiveTargetsLimit {
		return nil, errStationActiveTargetsUnavailable
	}
	var out stationActiveTargetsOutput
	if err := json.Unmarshal(body, &out); err != nil || out.Targets == nil || len(out.Targets) > statedb.MAX_ACTIVE_MULTISTREAM_TARGETS {
		return nil, errStationActiveTargetsUnavailable
	}
	for _, target := range out.Targets {
		if !validStationActiveTarget(did, target) {
			return nil, errStationActiveTargetsUnavailable
		}
	}
	return out.Targets, nil
}

// Run before user auth and wildcard forwarding: neither can receive this credential.
func (s *Server) stationActiveTargetsDispatch(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		if r.URL.Path != stationActiveTargetsPath {
			return next(c)
		}
		c.Response().Header().Set("Cache-Control", "private, no-store")
		if s.creatorRoutes == nil {
			return c.NoContent(http.StatusNotFound)
		}
		if r.Method != http.MethodGet {
			c.Response().Header().Set(echo.HeaderAllow, http.MethodGet)
			return c.NoContent(http.StatusMethodNotAllowed)
		}
		headers := r.Header.Values(stationActiveTargetsAuth)
		if len(headers) != 1 || r.URL.EscapedPath() != stationActiveTargetsPath || r.URL.ForceQuery || r.URL.RawQuery != "" ||
			(r.TLS == nil && !s.cli.BehindHTTPSProxy) {
			return c.NoContent(http.StatusForbidden)
		}
		key := stationActiveTargetsKey(s.cli)
		if key == nil {
			return c.NoContent(http.StatusForbidden)
		}
		var claims serviceAuthClaims
		node := s.cli.ServerDID()
		_, err := jwt.ParseWithClaims(headers[0], &claims, func(*jwt.Token) (any, error) { return key, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithAudience(node), jwt.WithExpirationRequired())
		if err != nil || len(claims.Audience) != 1 || claims.Lxm != stationActiveTargetsMethod ||
			claims.Issuer == node || s.creatorRoutes.peers[claims.Issuer] == nil {
			return c.NoContent(http.StatusForbidden)
		}
		if _, err := syntax.ParseDID(claims.Subject); err != nil {
			return c.NoContent(http.StatusForbidden)
		}
		ctx, did := r.Context(), claims.Subject
		owner, err := s.creatorRoutes.slots.Owner(ctx, did)
		if err != nil && !errors.Is(err, stationshard.ErrNotFound) {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		if err != nil || owner != node {
			return c.NoContent(http.StatusConflict)
		}
		targets, err := readStationActiveMultistreamTargets(ctx, s.statefulDB, did)
		if err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		return c.JSON(http.StatusOK, stationActiveTargetsOutput{Targets: targets})
	}
}

func readStationActiveMultistreamTargets(ctx context.Context, state *statedb.StatefulDB, did string) ([]placestream.MultistreamDefs_TargetView, error) {
	session, err := state.GetSpaceSession(did)
	if err != nil {
		return nil, errStationActiveTargetsUnavailable
	}
	if session != nil {
		if state.OATProxy == nil {
			return nil, errStationActiveTargetsUnavailable
		}
		session, err = state.OATProxy.RefreshIfNeeded(session)
		if err != nil || (session != nil && session.DID != did) {
			return nil, errStationActiveTargetsUnavailable
		}
	}
	var targets []placestream.MultistreamDefs_TargetView
	if session != nil && spaces.CollectionGrant(session.UpstreamScope, did, spaces.TargetCollection) &&
		(session.Status() == oatproxy.OAuthSessionStateReady || session.Status() == oatproxy.OAuthSessionStateReadyUpstream) {
		if session.UpstreamAccessToken == "" || session.UpstreamAccessTokenExp == nil || !time.Now().Before(*session.UpstreamAccessTokenExp) {
			return nil, errStationActiveTargetsUnavailable
		}
		client, err := state.OATProxy.GetXrpcClient(session)
		if err != nil {
			return nil, errStationActiveTargetsUnavailable
		}
		if err := state.StationMoveTargets(ctx, client, did); err != nil {
			return nil, errStationActiveTargetsUnavailable
		}
		targets, err = spaces.ListMultistreamTargets(ctx, client, did)
		if err != nil {
			return nil, errStationActiveTargetsUnavailable
		}
	} else {
		targets, err = state.StationListTargets(ctx, did)
		if errors.Is(err, statedb.ErrStationTargetsInSpace) {
			// A moved shard without a grant has no readable targets. Do not keep driving them.
			return []placestream.MultistreamDefs_TargetView{}, nil
		}
		if err != nil {
			return nil, errStationActiveTargetsUnavailable
		}
	}
	active := make([]placestream.MultistreamDefs_TargetView, 0, min(len(targets), statedb.MAX_ACTIVE_MULTISTREAM_TARGETS))
	for _, target := range targets {
		if validStationActiveTarget(did, target) {
			target.LatestEvent = nil
			active = append(active, target)
			if len(active) == statedb.MAX_ACTIVE_MULTISTREAM_TARGETS {
				break
			}
		}
	}
	return active, nil
}

func validStationActiveTarget(did string, target placestream.MultistreamDefs_TargetView) bool {
	if target.Record == nil {
		return false
	}
	record, ok := target.Record.Val.(*placestream.MultistreamTarget)
	if !ok || record == nil || !record.Active {
		return false
	}
	u, err := url.Parse(record.Url)
	if err != nil || u.Hostname() == "" || !slices.Contains(allowedSchemes, u.Scheme) || (u.Scheme == "rtmps" && u.Port() == "") {
		return false
	}
	uri, err := syntax.ParseATURI(target.Uri)
	return err == nil && uri.Authority().String() == did &&
		uri.Collection().String() == spaces.TargetCollection && uri.RecordKey() != ""
}
