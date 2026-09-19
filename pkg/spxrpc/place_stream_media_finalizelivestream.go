package spxrpc

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// handlePlaceStreamMediaFinalizeLivestream turns a recorded livestream (or
// several records of one recording) into a VOD. It creates a synthetic
// Upload row and enqueues a background finalize task that concatenates the
// recorded MUXL objects into a content blob and publishes the track
// records. By default it also creates a draft VOD in the 'processing' state
// that the streamer publishes later from the Drafts tab; with publish set,
// the task publishes the place.stream.video record itself as soon as the
// VOD is finalized, with the streamer's stored session. The caller is the
// streamer or a moderator with livestream.manage from them.
func (s *Server) handlePlaceStreamMediaFinalizeLivestream(ctx context.Context, body *placestream.MediaFinalizeLivestream_Input) (*placestream.MediaFinalizeLivestream_Output, error) {
	uris := uniqueURIs(body.Livestream, body.Livestreams)
	if len(uris) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "livestream or livestreams is required")
	}
	items, err := s.livestreamItems(uris)
	if err != nil {
		return nil, err
	}
	streamer := items[0].ls.RepoDID
	ordered := make([]string, len(items))
	for i, it := range items {
		ordered[i] = it.ls.URI
	}
	modCtx, err := s.requireLivestreamManage(ctx, streamer, "finalizeLivestream")
	if err != nil {
		return nil, err
	}
	ctx = log.WithLogValues(ctx, "func", "finalizeLivestream", "did", streamer, "by", modCtx.ModeratorDID, "livestreams", strings.Join(ordered, ","))

	// A banned account can't mint new VODs (the playback gates would hide them
	// regardless, but skip the work and the dead records).
	if banned, err := s.accountBanned(streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return nil, echo.NewHTTPError(http.StatusForbidden, "account is not permitted to publish videos")
	}

	segs, err := s.statefulDB.ListS3SegmentsForLivestreams(ctx, ordered)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "list recorded objects: "+err.Error())
	}
	if len(segs) == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "NoRecording: no completed recording objects for these livestreams")
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
		RepoDID:  streamer,
		MimeType: "video/mp4",
		Backend:  "live",
		Location: ordered[0],
	}); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	publish := body.Publish != nil && *body.Publish
	task := statedb.FinalizeLivestreamVODTask{UploadID: uploadID, RepoDID: streamer, LivestreamURI: ordered[0], LivestreamURIs: ordered}
	out := &placestream.MediaFinalizeLivestream_Output{UploadId: uploadID, Livestreams: ordered}
	objects := int64(len(segs))
	out.Objects = &objects
	if publish {
		task.Publish = videoDraftForLivestreams(items, deref(body.Title), deref(body.Description))
	} else {
		// A draft VOD in the 'processing' state, inheriting the first
		// livestream's metadata and linking back to it; it reaches 'ready'
		// server-side and the streamer publishes it from the Drafts tab.
		draft, err := s.createLivestreamDraft(ctx, streamer, uploadID, items[0].ls)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		out.DraftUri = &draft.URI
	}
	if _, err := s.statefulDB.EnqueueTask(ctx, statedb.TaskFinalizeLivestreamVOD, task, statedb.WithTaskKey("finalize-vod:"+uploadID)); err != nil {
		if out.DraftUri != nil {
			_, _ = s.statefulDB.DeleteDraft(ctx, *out.DraftUri)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	log.Log(ctx, "livestream VOD queued", "uploadId", uploadID, "objects", len(segs), "publish", publish)

	for _, it := range items {
		if it.rec.EndedAt != nil {
			out.Ended = append(out.Ended, it.ls.URI)
			continue
		}
		if body.EndLivestream == nil || !*body.EndLivestream {
			continue
		}
		if err := s.statefulDB.EndLivestreamRecord(ctx, it.ls, it.rec); err != nil {
			log.Error(ctx, "could not end livestream record", "livestream", it.ls.URI, "error", err)
		} else {
			out.Ended = append(out.Ended, it.ls.URI)
		}
	}
	return out, nil
}
