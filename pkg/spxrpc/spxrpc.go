package spxrpc

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

type Server struct {
	e     *echo.Echo
	cli   *config.CLI
	model model.Model
}

func NewServer(cli *config.CLI, model model.Model) (*Server, error) {
	e := echo.New()
	s := &Server{
		e:     e,
		cli:   cli,
		model: model,
	}
	err := s.RegisterHandlersPlaceStream(e)
	if err != nil {
		return nil, err
	}
	err = s.RegisterHandlersAppBsky(e)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.e.ServeHTTP(w, r)
}
