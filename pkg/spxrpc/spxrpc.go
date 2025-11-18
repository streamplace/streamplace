package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

type Server struct {
	e            *echo.Echo
	cli          *config.CLI
	model        model.Model
	OGImageCache *cache.Cache
	ATSync       *atproto.ATProtoSynchronizer
	statefulDB   *statedb.StatefulDB
	bus          *bus.Bus
}

func NewServer(ctx context.Context, cli *config.CLI, model model.Model, statefulDB *statedb.StatefulDB, op *oatproxy.OATProxy, mdlw middleware.Middleware, atsync *atproto.ATProtoSynchronizer, bus *bus.Bus) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:            e,
		cli:          cli,
		model:        model,
		OGImageCache: cache.New(5*time.Minute, 10*time.Minute), // 5min TTL, 10min cleanup
		ATSync:       atsync,
		statefulDB:   statefulDB,
		bus:          bus,
	}
	e.Use(s.ErrorHandlingMiddleware())
	e.Use(s.ContextPreservingMiddleware())
	e.Use(echomiddleware.Handler("", mdlw))
	e.Use(op.OAuthMiddleware)
	err := s.RegisterHandlersPlaceStream(e)
	if err != nil {
		return nil, err
	}
	err = s.RegisterHandlersAppBsky(e)
	if err != nil {
		return nil, err
	}
	err = s.RegisterHandlersComAtproto(e)
	if err != nil {
		return nil, err
	}
	e.GET("/xrpc/_health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"version": cli.Build.Version})
	})
	e.GET("/xrpc/com.atproto.sync.subscribeRepos", s.handleComAtprotoSyncSubscribeRepos)
	e.GET("/xrpc/place.stream.live.subscribeSegments", s.handlePlaceStreamLiveSubscribeSegments)
	e.GET("/xrpc/*", s.HandleWildcard)
	e.POST("/xrpc/*", s.HandleWildcard)
	return s, nil
}

func (s *Server) isLocalPDS(ctx context.Context, repo string) (bool, string, error) {
	did, svc, _, err := resolveRepoService(ctx, repo)
	if err != nil {
		return false, "", fmt.Errorf("resolveRepoService: %w", err)
	}
	if did == s.cli.MyDID() {
		return true, svc, nil
	}
	return false, svc, nil
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

	log.Error(ctx, "making unauthenticated request", "url", u.String())

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

func (s *Server) ErrorHandlingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			if err == nil {
				return nil
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
