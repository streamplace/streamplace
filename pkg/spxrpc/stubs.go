package spxrpc

import (
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

func (s *Server) RegisterHandlersAppBsky(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersChatBsky(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersComAtproto(e *echo.Echo) error {
	return nil
}

func (s *Server) RegisterHandlersPlaceStream(e *echo.Echo) error {
	e.POST("/xrpc/place.stream.account.login", s.HandlePlaceStreamAccountLogin)
	return nil
}

func (s *Server) HandlePlaceStreamAccountLogin(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandlePlaceStreamAccountLogin")
	defer span.End()

	var body placestreamtypes.AccountLogin_Input
	if err := c.Bind(&body); err != nil {
		return err
	}
	var out *placestreamtypes.AccountDefs_LoginResponse
	var handleErr error
	// func (s *Server) handlePlaceStreamAccountLogin(ctx context.Context,body *placestreamtypes.AccountLogin_Input) (*placestreamtypes.AccountDefs_LoginResponse, error)
	out, handleErr = s.handlePlaceStreamAccountLogin(ctx, &body)
	if handleErr != nil {
		return handleErr
	}
	return c.JSON(200, out)
}

func (s *Server) RegisterHandlersToolsOzone(e *echo.Echo) error {
	return nil
}
