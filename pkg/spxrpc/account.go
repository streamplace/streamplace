package spxrpc

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamAccountLogin(ctx context.Context, body *placestreamtypes.AccountLogin_Input) (*placestreamtypes.AccountDefs_LoginResponse, error) {
	return atproto.Login(ctx, s.cli, body, s.model)
}

func (s *Server) handlePlaceStreamAccountOauthReturn(ctx context.Context, code string, iss string, state string) error {
	err := atproto.HandleOauthReturn(ctx, s.cli, code, iss, state, s.model)
	if err != nil {
		log.Error(ctx, "failed to handle OAuth return", "error", err)
		return err
	}
	return nil
}

func (s *Server) HandlePlaceStreamAccountOauthReturn(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamAccountOauthReturn")
	defer span.End()
	code := c.QueryParam("code")
	iss := c.QueryParam("iss")
	state := c.QueryParam("state")
	var handleErr error
	// func (s *Server) handlePlaceStreamAccountOauthReturn(ctx context.Context,code string,iss string,state string) (io.Reader, error)
	handleErr = s.handlePlaceStreamAccountOauthReturn(ctx, code, iss, state)
	if handleErr != nil {
		return handleErr
	}
	return c.Redirect(302, "https://longos.iameli.link/")
}
