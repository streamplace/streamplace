package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
	placestreamtypes "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
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

// multistreamSpace locks all multistream calls before syncing with the caller's grant.
// The caller holds the returned lock through its index writes and sync.
func (s *Server) multistreamSpace(ctx context.Context, did string) (spaces.Client, func(), error) {
	unlock, err := s.statefulDB.GetNamedLock("multistream-space:" + did)
	if err != nil {
		return nil, nil, fmt.Errorf("lock multistream space: %w", err)
	}
	caller := GetCaller(ctx)
	if caller == nil || caller.OAuth == nil || s.op == nil {
		return nil, unlock, nil
	}
	session, err := s.op.RefreshIfNeeded(caller.OAuth)
	if err != nil || session == nil {
		log.Debug(ctx, "could not refresh multistream space session", "error", err)
		return nil, unlock, nil
	}
	// A PDS with spaces lists the expanded space scope in each token response.
	if !strings.Contains(session.GrantedScope(), "space:"+spaces.PersonalSpaceType) {
		return nil, unlock, nil
	}
	client, err := s.op.GetXrpcClient(session)
	if err != nil {
		log.Debug(ctx, "could not get multistream space client", "error", err)
		return nil, unlock, nil
	}
	if err := s.statefulDB.SyncMultistreamTargets(ctx, client, did); err != nil {
		log.Warn(ctx, "could not sync multistream targets", "error", err)
	}
	return client, unlock, nil
}

func (s *Server) handlePlaceStreamMultistreamCreateTarget(ctx context.Context, body *placestreamtypes.MultistreamCreateTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	client, unlock, err := s.multistreamSpace(ctx, session.DID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	err = validateMultistreamTargetURL(body.MultistreamTarget.Url)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	out, err := s.statefulDB.CreateMultistreamTarget(*body, session.DID)
	if err != nil {
		return nil, err
	}
	if err := s.statefulDB.SyncMultistreamTargets(ctx, client, session.DID); err != nil {
		log.Warn(ctx, "could not sync multistream targets", "error", err)
	}
	return &out, nil
}

func (s *Server) handlePlaceStreamMultistreamListTargets(ctx context.Context, cursor string, limit int) (*placestreamtypes.MultistreamListTargets_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamListTargets")
	defer span.End()

	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	_, unlock, err := s.multistreamSpace(ctx, session.DID)
	if err != nil {
		return nil, err
	}
	defer unlock()
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

	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	client, unlock, err := s.multistreamSpace(ctx, session.DID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	err = validateMultistreamTargetURL(body.MultistreamTarget.Url)
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
	if err := s.statefulDB.SyncMultistreamTargets(ctx, client, session.DID); err != nil {
		log.Warn(ctx, "could not sync multistream targets", "error", err)
	}
	return &out, nil
}
func (s *Server) handlePlaceStreamMultistreamDeleteTarget(ctx context.Context, body *placestreamtypes.MultistreamDeleteTarget_Input) (*placestreamtypes.MultistreamDeleteTarget_Output, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamDeleteTarget")
	defer span.End()

	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	client, unlock, err := s.multistreamSpace(ctx, session.DID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	// Build URI from rkey
	uri := fmt.Sprintf("at://%s/place.stream.multistream.target/%s", session.DID, body.Rkey)

	err = s.statefulDB.DeleteMultistreamTarget(ctx, client, uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to delete multistream target")
	}

	return &placestreamtypes.MultistreamDeleteTarget_Output{}, nil
}
