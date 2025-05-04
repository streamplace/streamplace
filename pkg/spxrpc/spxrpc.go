package spxrpc

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/config"
)

type Server struct {
	e   *echo.Echo
	cli *config.CLI
}

func NewServer(cli *config.CLI) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:   e,
		cli: cli,
	}
	err := s.RegisterHandlersPlaceStream(e)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.e.ServeHTTP(w, r)
}
