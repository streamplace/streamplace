package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
	placestreamtypes "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
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

// multistreamSpace returns a client for the caller's personal space, having
// moved their statedb targets into it. It is nil when the caller's session has
// no space grant, and statedb holds their targets.
func (s *Server) multistreamSpace(ctx context.Context) (spaces.Client, error) {
	caller := GetCaller(ctx)
	if caller.OAuth == nil || s.op == nil {
		return nil, nil
	}
	session, err := s.op.RefreshIfNeeded(caller.OAuth)
	if err != nil || session == nil || !spaces.Granted(session.GrantedScope()) {
		return nil, nil
	}
	client, err := s.op.GetXrpcClient(session)
	if err != nil {
		log.Debug(ctx, "could not get multistream space client", "error", err)
		return nil, nil
	}
	if err := s.statefulDB.MoveMultistreamTargets(ctx, client, caller.DID); err != nil {
		return nil, fmt.Errorf("move multistream targets to space: %w", err)
	}
	return client, nil
}

// putSpaceTarget creates a target in the space, or with an rkey replaces the
// one there, within the limits statedb keeps.
func putSpaceTarget(ctx context.Context, client spaces.Client, did, rkey string, target *placestreamtypes.MultistreamTarget) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	targets, err := spaces.ListMultistreamTargets(ctx, client, did)
	if err != nil {
		return nil, err
	}
	if rkey == "" {
		if len(targets) >= statedb.MAX_MULTISTREAM_TARGETS {
			return nil, fmt.Errorf("maximum number of multistream targets (%d) reached", statedb.MAX_MULTISTREAM_TARGETS)
		}
		rkey = spid.TIDClock.Next().String()
	} else if !slices.ContainsFunc(targets, func(t placestreamtypes.MultistreamDefs_TargetView) bool {
		return t.Uri == spaces.TargetURI(did, rkey)
	}) {
		return nil, fmt.Errorf("multistream target not found")
	}
	uri := spaces.TargetURI(did, rkey)
	if target.Active {
		active := 0
		for _, t := range targets {
			if t.Uri != uri && t.Record.Val.(*placestreamtypes.MultistreamTarget).Active {
				active++
			}
		}
		if active >= statedb.MAX_ACTIVE_MULTISTREAM_TARGETS {
			return nil, fmt.Errorf("maximum number of active multistream targets (%d) reached", statedb.MAX_ACTIVE_MULTISTREAM_TARGETS)
		}
	}
	_, cid, err := spaces.PutRecord(ctx, client, did, spaces.TargetCollection, rkey, target)
	if err != nil {
		return nil, err
	}
	return &placestreamtypes.MultistreamDefs_TargetView{
		Uri:    uri,
		Cid:    cid,
		Record: &glex.LexiconTypeDecoder{Val: target},
	}, nil
}

func (s *Server) handlePlaceStreamMultistreamCreateTarget(ctx context.Context, body *placestreamtypes.MultistreamCreateTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handleComAtprotoRepoUploadBlob")
	defer span.End()

	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	err := validateMultistreamTargetURL(body.MultistreamTarget.Url)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	client, err := s.multistreamSpace(ctx)
	if err != nil {
		return nil, err
	}
	if client != nil {
		return putSpaceTarget(ctx, client, session.DID, "", &body.MultistreamTarget)
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

	session := GetCaller(ctx)
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

	client, err := s.multistreamSpace(ctx)
	if err != nil {
		return nil, err
	}
	var targets []placestreamtypes.MultistreamDefs_TargetView
	if client != nil {
		targets, err = s.listSpaceTargets(ctx, client, session.DID, limit+1, offset)
	} else {
		targets, err = s.statefulDB.ListMultistreamTargets(session.DID, limit+1, offset, nil)
	}
	if err != nil {
		log.Error(ctx, "failed to list multistream targets", "error", err)
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

// listSpaceTargets pages through the space's targets with their latest events from statedb.
func (s *Server) listSpaceTargets(ctx context.Context, client spaces.Client, did string, limit, offset int) ([]placestreamtypes.MultistreamDefs_TargetView, error) {
	targets, err := spaces.ListMultistreamTargets(ctx, client, did)
	if err != nil {
		return nil, err
	}
	targets = targets[min(offset, len(targets)):]
	targets = targets[:min(limit, len(targets))]
	uris := make([]string, len(targets))
	for i, target := range targets {
		uris[i] = target.Uri
	}
	events, err := s.statefulDB.LatestMultistreamEvents(uris)
	if err != nil {
		return nil, err
	}
	for i := range targets {
		targets[i].LatestEvent = events[targets[i].Uri]
	}
	return targets, nil
}

func (s *Server) handlePlaceStreamMultistreamPutTarget(ctx context.Context, body *placestreamtypes.MultistreamPutTarget_Input) (*placestreamtypes.MultistreamDefs_TargetView, error) {
	ctx, span := otel.Tracer("server").Start(ctx, "handlePlaceStreamMultistreamPutTarget")
	defer span.End()

	session := GetCaller(ctx)
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

	client, err := s.multistreamSpace(ctx)
	if err != nil {
		return nil, err
	}
	if client != nil {
		return putSpaceTarget(ctx, client, session.DID, rkey, &body.MultistreamTarget)
	}
	out, err := s.statefulDB.UpdateMultistreamTarget(spaces.TargetURI(session.DID, rkey), *body)
	if err != nil {
		return nil, err
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

	client, err := s.multistreamSpace(ctx)
	if err != nil {
		return nil, err
	}
	if client != nil {
		err = spaces.DeleteRecord(ctx, client, session.DID, spaces.TargetCollection, body.Rkey)
	} else {
		err = s.statefulDB.DeleteMultistreamTarget(spaces.TargetURI(session.DID, body.Rkey))
	}
	if err != nil {
		log.Error(ctx, "failed to delete multistream target", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to delete multistream target")
	}

	return &placestreamtypes.MultistreamDeleteTarget_Output{}, nil
}
