package spxrpc

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// handlePlaceStreamMediaFinalizeLivestream turns a finished livestream into a
// VOD. It creates a synthetic Upload row and enqueues a background finalize
// task that concatenates the livestream's recorded MUXL objects into a content
// blob and publishes the track records. It also creates a draft VOD in the
// 'processing' state (inheriting the livestream's metadata) that the user
// publishes later from the Drafts tab via place.stream.vod.publishDraft — the
// client no longer polls getUploadStatus or calls publishVideo. Returns the
// draft's ats:// URI so the client can navigate to it.
func (s *Server) handlePlaceStreamMediaFinalizeLivestream(ctx context.Context, body *placestream.MediaFinalizeLivestream_Input) (*placestream.MediaFinalizeLivestream_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	ctx = log.WithLogValues(ctx, "func", "finalizeLivestream", "did", session.DID, "livestream", body.Livestream)
	if body.Livestream == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "livestream is required")
	}
	// A banned account can't mint new VODs (the playback gates would hide them
	// regardless, but skip the work and the dead records).
	if banned, err := s.accountBanned(session.DID); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "account is not permitted to publish videos")
	}

	ls, err := s.model.GetLivestream(body.Livestream)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if ls == nil || ls.Author.Did != session.DID {
		// Don't distinguish "not found" from "not yours" — same response.
		return nil, echo.NewHTTPError(http.StatusNotFound, "livestream not found")
	}

	// Synthetic Upload row so the client reuses the getUploadStatus /
	// publishVideo flow it already has for resumable uploads.
	uu, err := uuid.NewV7()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	uploadID := uu.String()
	if err := s.statefulDB.CreateUpload(ctx, &statedb.Upload{
		ID:       uploadID,
		RepoDID:  session.DID,
		MimeType: "video/mp4",
		Backend:  "live",
		Location: body.Livestream,
	}); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Create a draft VOD in the 'processing' state, inheriting the livestream's
	// metadata and linking back to it via connections. The client no longer polls
	// or publishes — the draft reaches 'ready' server-side and the user
	// publishes it later from the Drafts tab.
	draft, err := s.createLivestreamDraft(ctx, session.DID, uploadID, ls)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if _, err := s.statefulDB.EnqueueTask(ctx, statedb.TaskFinalizeLivestreamVOD, statedb.FinalizeLivestreamVODTask{
		UploadID:      uploadID,
		RepoDID:       session.DID,
		LivestreamURI: body.Livestream,
	}, statedb.WithTaskKey("finalize-vod:"+uploadID)); err != nil {
		_, _ = s.statefulDB.DeleteDraft(ctx, draft.URI)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return &placestream.MediaFinalizeLivestream_Output{UploadId: uploadID, DraftUri: draft.URI}, nil
}
