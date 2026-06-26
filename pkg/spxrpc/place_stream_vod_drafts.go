package spxrpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	placestream "stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/pkg/vod"
)

// draft helpers ─────────────────────────────────────────────────────────────

// draftNotFound and draftForbidden map the common ownership/not-found case to a
// 404, so a caller can't probe another user's draft URIs.
func draftNotFound() error {
	return echo.NewHTTPError(http.StatusNotFound, "draft not found")
}

// loadOwnedDraft fetches a draft by URI and verifies it belongs to session.DID.
// Returns a 404 (draftNotFound) whether the draft is absent or belongs to
// someone else, so a caller can't probe another user's draft URIs.
func (s *Server) loadOwnedDraft(ctx context.Context, uri, did string) (*placestream.VodDraftDefs_DraftView, error) {
	if uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	dv, err := s.statefulDB.GetDraft(ctx, uri)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if dv == nil || dv.UserDID != did {
		return nil, draftNotFound()
	}
	view, err := dv.ToDraftView()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return view, nil
}

// handlers ─────────────────────────────────────────────────────────────────

func (s *Server) handlePlaceStreamVodListDrafts(ctx context.Context, cursor string, limit int) (*placestream.VodListDrafts_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	drafts, err := s.statefulDB.ListDrafts(ctx, session.DID, limit, cursor)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	views := make([]*placestream.VodDraftDefs_DraftView, 0, len(drafts))
	for _, dv := range drafts {
		v, err := dv.ToDraftView()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		views = append(views, v)
	}
	out := &placestream.VodListDrafts_Output{Drafts: views}
	if len(drafts) > 0 {
		c := drafts[len(drafts)-1].CreatedAt.Format(time.RFC3339Nano)
		out.Cursor = &c
	}
	return out, nil
}

func (s *Server) handlePlaceStreamVodGetDraft(ctx context.Context, uri string) (*placestream.VodGetDraft_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	owned, err := s.loadOwnedDraft(ctx, uri, session.DID)
	if err != nil {
		return nil, err
	}
	return &placestream.VodGetDraft_Output{Draft: owned}, nil
}

func (s *Server) handlePlaceStreamVodUpdateDraft(ctx context.Context, body *placestream.VodUpdateDraft_Input) (*placestream.VodUpdateDraft_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	if body.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	// Verify ownership before mutating.
	if _, err := s.loadOwnedDraft(ctx, body.Uri, session.DID); err != nil {
		return nil, err
	}
	// Apply only the editable fields present in the partial input. The server-
	// authoritative fields (source, durationMs, status, error) are never touched
	// here — the closure only mutates editable metadata.
	updated, err := s.statefulDB.UpdateDraftMetadata(ctx, body.Uri, func(rec *placestream.VodDraftVideo) {
		if body.Title != nil {
			rec.Title = *body.Title
		}
		if body.Description != nil {
			rec.Description = body.Description
		}
		// descriptionFacets / tags: nil means "not provided" (leave as-is); an
		// empty slice is a deliberate clear, so always overwrite when non-nil.
		if body.DescriptionFacets != nil {
			rec.DescriptionFacets = body.DescriptionFacets
		}
		if body.Tags != nil {
			rec.Tags = body.Tags
		}
		if body.Thumb != nil {
			rec.Thumb = body.Thumb
		}
		if body.Activity != nil {
			rec.Activity = (*placestream.VodDraftVideo_Activity)(body.Activity)
		}
		if body.ContentWarnings != nil {
			rec.ContentWarnings = body.ContentWarnings
		}
		if body.ContentRights != nil {
			rec.ContentRights = body.ContentRights
		}
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if updated == nil {
		return nil, draftNotFound()
	}
	view, err := updated.ToDraftView()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return &placestream.VodUpdateDraft_Output{Draft: view}, nil
}

func (s *Server) handlePlaceStreamVodDeleteDraft(ctx context.Context, body *placestream.VodDeleteDraft_Input) error {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	if body.Uri == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	// Verify ownership before deleting.
	if _, err := s.loadOwnedDraft(ctx, body.Uri, session.DID); err != nil {
		return err
	}
	deleted, err := s.statefulDB.DeleteDraft(ctx, body.Uri)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if !deleted {
		return draftNotFound()
	}
	return nil
}

func (s *Server) handlePlaceStreamVodPublishDraft(ctx context.Context, body *placestream.VodPublishDraft_Input) (*placestream.VodPublishDraft_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
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

	videoURI, videoCID, err := vod.PublishDraft(ctx, s.statefulDB, s.playbackStore, session.DID, body.Uri)
	if err != nil {
		switch {
		case errors.Is(err, vod.ErrDraftNotFound):
			return nil, draftNotFound()
		case errors.Is(err, vod.ErrDraftNotReady):
			return nil, echo.NewHTTPError(http.StatusConflict, "draft is not ready to publish")
		default:
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	return &placestream.VodPublishDraft_Output{VideoUri: videoURI, VideoCid: videoCID}, nil
}
