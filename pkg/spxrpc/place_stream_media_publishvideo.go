package spxrpc

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// handlePlaceStreamMediaPublishVideo creates a place.stream.video record in
// the authenticated user's repo for a finished upload. The client hands us
// the record it would otherwise putRecord itself; the server overrides the

// handlePlaceStreamMediaPublishVideo publishes a place.stream.video record
// for a finished upload of the streamer's: the record the caller supplies,
// or one built from the upload's livestream(s) and the given title and
// description (what the finalize does after the finalize, on its own, for an
// upload finalized without publish, a publish that failed, or a record that
// was deleted). The caller is the upload's owner or a moderator with
// livestream.manage from them; the record is written with the owner's
// session.
func (s *Server) handlePlaceStreamMediaPublishVideo(ctx context.Context, body *placestream.MediaPublishVideo_Input) (*placestream.MediaPublishVideo_Output, error) {
	if body.UploadId == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uploadId is required")
	}
	if s.playbackStore == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	upload, err := s.statefulDB.GetUpload(ctx, body.UploadId)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "get upload: "+err.Error())
	}
	if upload == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "upload not found")
	}
	modCtx, err := s.requireLivestreamManage(ctx, upload.RepoDID, "publishVideo")
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "func", "publishVideo", "did", upload.RepoDID, "by", modCtx.ModeratorDID, "uploadId", body.UploadId)

	record := body.Record
	if record == nil {
		uris := body.Livestreams
		if len(uris) == 0 && strings.HasPrefix(upload.Location, "at://") {
			uris = []string{upload.Location}
		}
		var items []livestreamItem
		if len(uris) > 0 {
			if items, err = s.livestreamItems(uris); err != nil {
				return nil, err
			}
			if items[0].ls.RepoDID != upload.RepoDID {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "the livestreams belong to a different streamer than the upload")
			}
		}
		var draft *statedb.VideoDraft
		if len(items) > 0 {
			draft = videoDraftForLivestreams(items, deref(body.Title), deref(body.Description))
		} else {
			title := strings.TrimSpace(deref(body.Title))
			if title == "" {
				title = "Video"
			}
			draft = &statedb.VideoDraft{Title: title}
			if d := strings.TrimSpace(deref(body.Description)); d != "" {
				draft.Description = &d
			}
		}
		record = draft.Record()
	}

	uri, cid, err := vod.PublishVideo(ctx, s.statefulDB, s.playbackStore, upload.RepoDID, upload.ID, record)
	if err != nil {
		return nil, videoError("publish video", err)
	}
	log.Log(ctx, "video record published", "uri", uri, "cid", cid)
	return &placestream.MediaPublishVideo_Output{Uri: uri, Cid: cid}, nil
}
