package spxrpc

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

type Server struct {
	e     *echo.Echo
	cli   *config.CLI
	model model.Model
}

func NewServer(ctx context.Context, cli *config.CLI, model model.Model, op *oatproxy.OATProxy, mdlw middleware.Middleware) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:     e,
		cli:   cli,
		model: model,
	}
	e.Use(s.ErrorHandlingMiddleware())
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
