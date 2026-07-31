package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"go.opentelemetry.io/otel"
	placestreamtypes "stream.place/streamplace/pkg/placestream"
)

var allowedSchemes = []string{"rtmp", "rtmps"}

func validateMultistreamTargetURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid multistream target URL: %w", err)
	}
	if !slices.Contains(allowedSchemes, u.Scheme) {
		return fmt.Errorf("invalid multistream target scheme (must be rtmp or rtmps)")
	}
	if u.Scheme == "rtmps" && u.Port() == "" {
		return fmt.Errorf("rtmps URLs must include a port")
	}
	return nil
}

func (s *Server) handlePlaceStreamMultistreamCreateTarget(ctx context.Context, body *placestreamtypes.MultistreamCreateTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	err := validateMultistreamTargetURL(body.MultistreamTarget.Url)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	out, err := s.statefulDB.CreateMultistreamTarget(*body, session.DID)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Server) handlePlaceStreamMultistreamListTargets(ctx context.Context, cursor string, limit int) (*placestreamtypes.MultistreamListTargets_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamListTargets")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// Set default limit and validate bounds
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Parse cursor for offset
	offset := 0
	if cursor != "" {
		var err error
		offset, err = strconv.Atoi(cursor)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
		}
	}

	targets, err := s.statefulDB.ListMultistreamTargets(session.DID, limit+1, offset, nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to list multistream targets")
	}

	// Check if there are more results
	var nextCursor *string
	if len(targets) > limit {
		targets = targets[:limit]
		next := strconv.Itoa(offset + limit)
		nextCursor = &next
	}

	return &placestreamtypes.MultistreamListTargets_Output{
		Targets: targets,
		Cursor:  nextCursor,
	}, nil
}
func (s *Server) handlePlaceStreamMultistreamPutTarget(ctx context.Context, body *placestreamtypes.MultistreamPutTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamPutTarget")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	err := validateMultistreamTargetURL(body.MultistreamTarget.Url)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Build URI from rkey
	rkey := ""
	if body.Rkey != nil {
		rkey = *body.Rkey
	}
	if rkey == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "rkey is required")
	}
	uri := fmt.Sprintf("at://%s/place.stream.multistream.target/%s", session.DID, rkey)

	out, err := s.statefulDB.UpdateMultistreamTarget(uri, *body)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func (s *Server) handlePlaceStreamMultistreamDeleteTarget(ctx context.Context, body *placestreamtypes.MultistreamDeleteTarget_Input) (*placestreamtypes.MultistreamDeleteTarget_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamDeleteTarget")
	defer span.End()

	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// Build URI from rkey
	uri := fmt.Sprintf("at://%s/place.stream.multistream.target/%s", session.DID, body.Rkey)

	err := s.statefulDB.DeleteMultistreamTarget(uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to delete multistream target")
	}

	return &placestreamtypes.MultistreamDeleteTarget_Output{}, nil
}
