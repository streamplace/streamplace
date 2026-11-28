package spxrpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

func draftHTTPError(err error) error {
	switch {
	case errors.Is(err, statedb.ErrDraftNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "draft not found")
	case errors.Is(err, statedb.ErrDraftPublished):
		return echo.NewHTTPError(http.StatusConflict, statedb.ErrDraftPublished.Error())
	case errors.Is(err, statedb.ErrDraftReauthorize), spaces.DraftAuthorizationError(err):
		return echo.NewHTTPError(http.StatusForbidden, "Reauthorize: sign in again to access private drafts")
	case errors.Is(err, statedb.ErrDraftCursor):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, vod.ErrDraftNotReady):
		return echo.NewHTTPError(http.StatusConflict, "draft is not ready to publish")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "draft operation failed")
	}
}

func draftCaller(ctx context.Context) (string, *oatproxy.OAuthSession, *oatproxy.XrpcClient) {
	if session, client := oatproxy.GetOAuthSession(ctx); session != nil {
		// OAuthMiddleware already refreshed this exact request session.
		return session.DID, session, client
	}
	did, _ := ctx.Value(pdsAuthContextKey).(string)
	return did, nil, nil
}

func (s *Server) openDrafts(ctx context.Context, did string, session *oatproxy.OAuthSession, client *oatproxy.XrpcClient, actions ...string) (*statedb.Drafts, error) {
	if did == "" {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	var privateClient spaces.Client
	bound := session != nil && session.DID == did
	var permissions spaces.CollectionPermissions
	if bound {
		permissions = spaces.ParseCollectionPermissions(session.UpstreamScope, did, spaces.DraftMetadataCollection)
	}
	if permissions.Full() {
		if client == nil {
			return nil, draftHTTPError(statedb.ErrDraftReauthorize)
		}
		privateClient = client
	} else if permissions != 0 || creatorSpaceOnly(ctx) {
		// Standalone has no creator routing middleware. A partial draft
		// grant is still a private-space credential, not a SQL credential.
		if client == nil || !permissions.Allows(actions...) {
			return nil, draftHTTPError(statedb.ErrDraftReauthorize)
		}
		drafts, err := s.statefulDB.OpenMigratedDrafts(ctx, did, client)
		if err != nil {
			return nil, draftHTTPError(err)
		}
		return drafts, nil
	}
	drafts, err := s.statefulDB.OpenDrafts(ctx, did, privateClient)
	if err != nil {
		return nil, draftHTTPError(err)
	}
	return drafts, nil
}

func (s *Server) openCallerDrafts(ctx context.Context, actions ...string) (*statedb.Drafts, error) {
	did, session, client := draftCaller(ctx)
	return s.openDrafts(ctx, did, session, client, actions...)
}

func (s *Server) handlePlaceStreamVodListDrafts(ctx context.Context, cursor string, limit int) (*placestream.VodListDrafts_Output, error) {
	drafts, err := s.openCallerDrafts(ctx, "read")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	rows, next, err := drafts.List(ctx, limit, cursor)
	if err != nil {
		return nil, draftHTTPError(err)
	}
	views := make([]placestream.VodDraftDefs_DraftView, 0, len(rows))
	for _, dv := range rows {
		v, err := dv.ToDraftView()
		if err != nil {
			return nil, draftHTTPError(err)
		}
		views = append(views, v)
	}
	out := placestream.VodListDrafts_Output{Drafts: views}
	if next != "" {
		out.Cursor = &next
	}
	return &out, nil
}

func (s *Server) handlePlaceStreamVodGetDraft(ctx context.Context, uri string) (*placestream.VodGetDraft_Output, error) {
	if uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	drafts, err := s.openCallerDrafts(ctx, "read")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	dv, err := drafts.Get(ctx, uri)
	if err != nil {
		return nil, draftHTTPError(err)
	}
	view, err := dv.ToDraftView()
	if err != nil {
		return nil, draftHTTPError(err)
	}
	return &placestream.VodGetDraft_Output{Draft: view}, nil
}

func (s *Server) handlePlaceStreamVodUpdateDraft(ctx context.Context, body *placestream.VodUpdateDraft_Input) (*placestream.VodUpdateDraft_Output, error) {
	if body.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	drafts, err := s.openCallerDrafts(ctx, "read", "update")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	updated, err := drafts.Update(ctx, body)
	if err != nil {
		return nil, draftHTTPError(err)
	}
	view, err := updated.ToDraftView()
	if err != nil {
		return nil, draftHTTPError(err)
	}
	return &placestream.VodUpdateDraft_Output{Draft: view}, nil
}

func (s *Server) handlePlaceStreamVodDeleteDraft(ctx context.Context, body *placestream.VodDeleteDraft_Input) (*placestream.VodDeleteDraft_Output, error) {
	if body.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	drafts, err := s.openCallerDrafts(ctx, "delete")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	if err := drafts.Delete(ctx, body.Uri); err != nil {
		return nil, draftHTTPError(err)
	}
	return &placestream.VodDeleteDraft_Output{}, nil
}

// handlePlaceStreamVodCreateDraft creates an empty 'processing' draft VOD — the
// durable anchor for an upload. The client creates it first, then passes the
// returned URI to place.stream.media.createUpload so the upload's processing
// fills this draft. Lets the user edit metadata while the upload runs and
// supports re-upload (a failed upload leaves the draft intact).
func (s *Server) handlePlaceStreamVodCreateDraft(ctx context.Context, body *placestream.VodCreateDraft_Input) (*placestream.VodCreateDraft_Output, error) {
	did, session, client := draftCaller(ctx)
	if did == "" {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	// A banned account can't create VOD drafts.
	if banned, err := s.accountBanned(did); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "account is not permitted to publish videos")
	}
	drafts, err := s.openDrafts(ctx, did, session, client, "create")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	dv, err := drafts.Create(ctx, "", &placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         "Untitled",
		Status:        "processing",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, draftHTTPError(err)
	}
	return &placestream.VodCreateDraft_Output{Uri: dv.URI}, nil
}

func (s *Server) handlePlaceStreamVodPublishDraft(ctx context.Context, body *placestream.VodPublishDraft_Input) (*placestream.VodPublishDraft_Output, error) {
	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	ctx = log.WithLogValues(ctx, "func", "publishDraft", "did", session.DID, "draftUri", body.Uri)
	if body.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	if s.playbackStore == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	if banned, err := s.accountBanned(session.DID); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "account is not permitted to publish videos")
	}

	drafts, err := s.openDrafts(ctx, session.DID, session, client, "read", "delete")
	if err != nil {
		return nil, err
	}
	defer drafts.Close()
	videoURI, videoCID, err := vod.PublishDraft(ctx, s.statefulDB, s.playbackStore, drafts, client, body.Uri)
	if err != nil {
		return nil, draftHTTPError(err)
	}
	return &placestream.VodPublishDraft_Output{VideoUri: videoURI, VideoCid: videoCID}, nil
}
