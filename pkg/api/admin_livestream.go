package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// finalizeLivestreamRequest is the body of POST /finalize-livestream on the
// internal API.
type finalizeLivestreamRequest struct {
	// Livestream is the at:// URI of the place.stream.livestream record whose
	// recording becomes the VOD (every recorded object under it, in order).
	Livestream string `json:"livestream"`
	// Title and Description of the video record; the livestream's title
	// when empty.
	Title       string `json:"title"`
	Description string `json:"description"`
	// Publish (default true) publishes the video record as soon as the VOD
	// is finalized; false leaves an upload the streamer publishes from the
	// app's Livestreams tab.
	Publish *bool `json:"publish"`
	// EndLivestream also sets endedAt on a record the streamer never
	// stopped, so their page stops reading as live.
	EndLivestream bool `json:"endLivestream"`
}

type finalizeLivestreamResponse struct {
	UploadID  string `json:"uploadId"`
	RepoDID   string `json:"repoDID"`
	Objects   int    `json:"objects"`
	Bytes     int64  `json:"bytes"`
	Publish   bool   `json:"publish"`
	Title     string `json:"title"`
	Ended     bool   `json:"ended"`
	EndError  string `json:"endError,omitempty"`
	TaskQueue string `json:"taskQueue"`
}

// HandleFinalizeLivestream (internal API, POST /finalize-livestream) turns a
// recorded livestream into a VOD and, by default, publishes it on the
// streamer's channel: the operator's counterpart of the app's Livestreams
// tab, for a stream whose streamer can't or won't click through (a client's
// event, a stream that was never stopped). The records are written with the
// streamer's stored OAuth session, as the app path does; nothing here can
// act for a streamer who never signed in to this node. Loopback only, like
// the rest of the internal API.
func (a *StreamplaceAPI) HandleFinalizeLivestream(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req finalizeLivestreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		if req.Livestream == "" {
			errors.WriteHTTPBadRequest(w, "livestream (at:// URI) is required", nil)
			return
		}
		ls, err := a.Model.GetLivestream(req.Livestream)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "get livestream", err)
			return
		}
		if ls == nil {
			errors.WriteHTTPNotFound(w, "livestream not indexed on this node", nil)
			return
		}
		view, err := ls.ToLivestreamView()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "decode livestream", err)
			return
		}
		rec, ok := view.Record.Val.(*placestream.Livestream)
		if !ok {
			errors.WriteHTTPInternalServerError(w, "livestream record is not a place.stream.livestream", nil)
			return
		}
		segs, err := a.StatefulDB.ListS3SegmentsForLivestream(ctx, ls.URI)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "list recorded objects", err)
			return
		}
		if len(segs) == 0 {
			errors.WriteHTTPNotFound(w, "no completed recording objects for this livestream", nil)
			return
		}
		var total int64
		for _, s := range segs {
			total += s.Size
		}
		if session, err := a.StatefulDB.GetSessionByDID(ls.RepoDID); err != nil || session == nil {
			errors.WriteHTTPBadRequest(w, "the streamer has no stored session on this node; they must sign in once before their records can be written", err)
			return
		}
		publish := req.Publish == nil || *req.Publish
		uu, err := uuid.NewV7()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "uuid", err)
			return
		}
		uploadID := uu.String()
		if err := a.StatefulDB.CreateUpload(ctx, &statedb.Upload{
			ID:       uploadID,
			RepoDID:  ls.RepoDID,
			MimeType: "video/mp4",
			Backend:  "live",
			Location: ls.URI,
		}); err != nil {
			errors.WriteHTTPInternalServerError(w, "create upload", err)
			return
		}
		task := statedb.FinalizeLivestreamVODTask{UploadID: uploadID, RepoDID: ls.RepoDID, LivestreamURI: ls.URI}
		video := videoRecordForLivestream(rec, ls, req.Title, req.Description)
		if publish {
			task.Publish = video
		}
		if _, err := a.StatefulDB.EnqueueTask(ctx, statedb.TaskFinalizeLivestreamVOD, task, statedb.WithTaskKey("finalize-vod:"+uploadID)); err != nil {
			errors.WriteHTTPInternalServerError(w, "enqueue finalize task", err)
			return
		}
		log.Log(ctx, "operator finalize: livestream VOD queued", "livestream", ls.URI, "repoDID", ls.RepoDID, "uploadId", uploadID, "objects", len(segs), "bytes", total, "publish", publish)
		resp := finalizeLivestreamResponse{UploadID: uploadID, RepoDID: ls.RepoDID, Objects: len(segs), Bytes: total, Publish: publish, Title: video.Title, TaskQueue: statedb.TaskFinalizeLivestreamVOD}
		if req.EndLivestream && rec.EndedAt == nil {
			if err := a.StatefulDB.EndLivestreamRecord(ctx, ls, rec); err != nil {
				log.Error(ctx, "operator finalize: could not end livestream record", "livestream", ls.URI, "error", err)
				resp.EndError = err.Error()
			} else {
				resp.Ended = true
			}
		} else if rec.EndedAt != nil {
			resp.Ended = true
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

// videoRecordForLivestream is the place.stream.video record for a
// livestream's VOD: its title (or the given one), description, tags and
// activity, connected to the livestream record the way the app's draft is.
// Duration, source tracks and thumbnail are filled in at publish time from
// the finalized upload.
func videoRecordForLivestream(rec *placestream.Livestream, ls *model.Livestream, title, description string) *placestream.Video {
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(rec.Title)
	}
	if title == "" {
		title = "Livestream"
	}
	v := &placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         title,
		Tags:          rec.Tags,
		Connections: []placestream.Video_Connections_Elem{{
			Video_Connection: &placestream.Video_Connection{
				LexiconTypeID: "place.stream.video#connection",
				Ref:           &comatproto.RepoStrongRef{Uri: ls.URI, Cid: ls.CID},
			},
		}},
	}
	if d := strings.TrimSpace(description); d != "" {
		v.Description = &d
	}
	if rec.Activity != nil {
		switch {
		case rec.Activity.Defs_ActivityGame != nil:
			v.Activity = &placestream.Video_Activity{Defs_ActivityGame: rec.Activity.Defs_ActivityGame}
		case rec.Activity.Defs_ActivityLabel != nil:
			v.Activity = &placestream.Video_Activity{Defs_ActivityLabel: rec.Activity.Defs_ActivityLabel}
		}
	}
	return v
}
