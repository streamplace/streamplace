package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/upload"
	"stream.place/streamplace/pkg/viewlog"
)

type Server struct {
	e               *echo.Echo
	cli             *config.CLI
	model           model.Model
	OGImageCache    *cache.Cache
	LiveUsersCache  *cache.Cache
	GameSearchCache *cache.Cache
	ScoreCache      *cache.Cache
	ATSync          *atproto.ATProtoSynchronizer
	statefulDB      *statedb.StatefulDB
	bus             *bus.Bus
	op              *oatproxy.OATProxy
	localDB         localdb.LocalDB
	mm              *media.MediaManager
	uploadManager   *upload.Manager
	// playbackStore is where VOD blobs + metafiles + per-track init
	// segments live. Matches the blob.Store the VOD processor writes
	// into (vod.BlobsPrefix + <cid>.{mp4,json}).
	playbackStore blob.Store
	// viewLog records playback request events (manifest + segment
	// fetches) for later view-count aggregation. Optional; nil when
	// --view-log-flush-interval is 0 or no playback store is wired.
	viewLog *viewlog.Writer
	aliases map[string]string
}

func NewServer(ctx context.Context, cli *config.CLI, model model.Model, statefulDB *statedb.StatefulDB, op *oatproxy.OATProxy, mdlw middleware.Middleware, atsync *atproto.ATProtoSynchronizer, bus *bus.Bus, ldb localdb.LocalDB, mm *media.MediaManager, um *upload.Manager, playbackStore blob.Store, viewLog *viewlog.Writer, aliases map[string]string) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:               e,
		cli:             cli,
		model:           model,
		OGImageCache:    cache.New(5*time.Minute, 10*time.Minute),
		LiveUsersCache:  cache.New(30*time.Second, 60*time.Second),
		GameSearchCache: cache.New(60*time.Second, 2*time.Minute),
		ScoreCache:      cache.New(30*time.Second, 60*time.Second),
		ATSync:          atsync,
		statefulDB:      statefulDB,
		bus:             bus,
		op:              op,
		localDB:         ldb,
		mm:              mm,
		uploadManager:   um,
		playbackStore:   playbackStore,
		viewLog:         viewLog,
		aliases:         aliases,
	}
	e.Use(s.ErrorHandlingMiddleware())
	e.Use(s.ContextPreservingMiddleware())
	e.Use(echomiddleware.Handler("", mdlw))
	e.Use(s.ServiceAuthMiddleware())
	e.Use(op.OAuthMiddleware)
	err := s.RegisterHandlersPlacestream(e)
	if err != nil {
		return nil, err
	}
	e.GET("/xrpc/_health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"version": cli.Build.Version})
	})
	e.GET("/xrpc/com.atproto.sync.subscribeRepos", s.handleComAtprotoSyncSubscribeRepos)
	e.GET("/xrpc/place.stream.live.subscribeSegments", s.handlePlaceStreamLiveSubscribeSegments)
	// Override the auto-generated playback stubs: the wrapper in
	// stubs.go hard-codes status 200 + content-type, but we need
	// 206 Partial Content for HTTP Range on getVideoBlob and the
	// `application/vnd.apple.mpegurl` MIME for getVideoPlaylist.
	// Registering AFTER RegisterHandlersPlacestream wins because
	// echo's last-write-wins for exact-match routes.
	e.GET("/xrpc/place.stream.playback.getVideoBlob", s.HandleGetVideoBlob)
	e.GET("/xrpc/place.stream.playback.getVideoPlaylist", s.HandleGetVideoPlaylist)
	// Live HLS, same override rationale (Range + mpegurl), served from the
	// in-memory live window instead of a stored metafile.
	e.GET("/xrpc/place.stream.playback.getLivePlaylist", s.HandleGetLivePlaylist)
	e.GET("/xrpc/place.stream.playback.getLiveSegment", s.HandleGetLiveSegment)
	e.GET("/xrpc/*", s.HandleWildcard)
	e.POST("/xrpc/*", s.HandleWildcard)
	return s, nil
}

func (s *Server) isLocalPDS(ctx context.Context, repo string) (bool, string, error) {
	did, svc, _, err := resolveRepoService(ctx, repo)
	if err != nil {
		return false, "", fmt.Errorf("resolveRepoService: %w", err)
	}
	if did == s.cli.BroadcasterDID() || did == s.cli.ServerDID() {
		return true, svc, nil
	}
	return false, svc, nil
}

// isServerPDS returns true if the request arrived on the ServerHost (server-local PDS).
// When ServerHost == BroadcasterHost (single-node), always returns true — the server
// PDS is the only PDS and the lexicon repo is not directly exposed.
func (s *Server) isServerPDS(ctx context.Context) bool {
	if s.cli.ServerHost == s.cli.BroadcasterHost {
		return true
	}
	ec, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return false
	}
	return ec.Request().Host == s.cli.ServerHost
}

func makeUnauthenticatedRequest(ctx context.Context, service, method string, params map[string]interface{}, out interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	u, err := url.Parse(fmt.Sprintf("%s/xrpc/%s", service, method))
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	// add query parameters
	query := u.Query()
	for k, v := range params {
		query.Set(k, fmt.Sprintf("%v", v))
	}
	u.RawQuery = query.Encode()

	log.Debug(ctx, "making unauthenticated request", "url", u.String())

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := aqhttp.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.e.ServeHTTP(w, r)
}

const AccountDeactivated = "AccountDeactivated"

func (s *Server) ErrorHandlingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err == nil {
				return nil
			}
			// this can mean we missed a PDS migration and need to refresh identity
			if strings.Contains(err.Error(), AccountDeactivated) {
				session, _ := oatproxy.GetOAuthSession(c.Request().Context())
				if session != nil {
					_, err := s.ATSync.RefreshIdentity(c.Request().Context(), session.DID)
					if err != nil {
						return err
					}
				}
			}
			httpError, ok := err.(*echo.HTTPError)
			if ok {
				log.Error(c.Request().Context(), "http error", "code", httpError.Code, "message", httpError.Message, "internal", httpError.Internal)
				return err
			}
			log.Error(c.Request().Context(), "unhandled error", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
}

// unique type to prevent assignment.
type echoContextKeyType struct{}

// singleton value to identify our logging metadata in context
var echoContextKey = echoContextKeyType{}

func (s *Server) ContextPreservingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, echoContextKey, c)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
