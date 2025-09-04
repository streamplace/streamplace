package spxrpc

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/atproto"
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
}

func NewServer(ctx context.Context, cli *config.CLI, model model.Model, statefulDB *statedb.StatefulDB, op *oatproxy.OATProxy, mdlw middleware.Middleware, atsync *atproto.ATProtoSynchronizer) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:            e,
		cli:          cli,
		model:        model,
		OGImageCache: cache.New(5*time.Minute, 10*time.Minute), // 5min TTL, 10min cleanup
		ATSync:       atsync,
		statefulDB:   statefulDB,
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
	e.GET("/xrpc/*", s.HandleWildcard)
	e.POST("/xrpc/*", s.HandleWildcard)
	return s, nil
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
