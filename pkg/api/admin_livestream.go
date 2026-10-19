package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// finalizeLivestreamRequest is the body of POST /finalize-livestream on the
// internal API.
type finalizeLivestreamRequest struct {
	// Livestream is the at:// URI of the place.stream.livestream record whose
	// recording becomes the VOD (every recorded object under it, in order).
	Livestream string `json:"livestream"`
	// Livestreams names several records whose recordings make up one VOD:
	// a streamer who started a new record mid-stream (some clients do that
	// on a title change) split the recording across them. Objects are
	// concatenated in recording order across all of them. Either field or
	// both may be given.
	Livestreams []string `json:"livestreams"`
	// Title and Description of the video record; the first livestream's
	// title when empty.
	Title       string `json:"title"`
	Description string `json:"description"`
	// Publish (default true) publishes the video record as soon as the VOD
	// is finalized; false leaves an upload the streamer publishes from the
	// app's Livestreams tab.
	Publish *bool `json:"publish"`
	// EndLivestream also sets endedAt on any of the records the streamer
	// never stopped, so their page stops reading as live.
	EndLivestream bool `json:"endLivestream"`
	// EndOnly ends the never-stopped records and does nothing else: no
	// upload, no VOD. For records left live after a finalize that already
	// ran.
	EndOnly bool `json:"endOnly"`
}

type finalizeLivestreamResponse struct {
	UploadID    string   `json:"uploadId"`
	RepoDID     string   `json:"repoDID"`
	Livestreams []string `json:"livestreams"`
	Objects     int      `json:"objects"`
	Bytes       int64    `json:"bytes"`
	Publish     bool     `json:"publish"`
	Title       string   `json:"title"`
	Ended       []string `json:"ended,omitempty"`
	EndErrors   []string `json:"endErrors,omitempty"`
	TaskQueue   string   `json:"taskQueue"`
}

// HandleFinalizeLivestream (internal API, POST /finalize-livestream) turns a
// recorded livestream into a VOD and, by default, publishes it on the
// streamer's channel: the operator's counterpart of the app's Livestreams
// tab, for a stream whose streamer can't or won't click through (a client's
// event, a stream that was never stopped), and the only path that can join
// a recording split across livestream records. The records are written with
// the streamer's stored OAuth session, as the app path does; nothing here
// can act for a streamer who never signed in to this node. Loopback only,
// like the rest of the internal API.
func (a *StreamplaceAPI) HandleFinalizeLivestream(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req finalizeLivestreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		var uris []string
		if req.Livestream != "" {
			uris = append(uris, req.Livestream)
		}
		for _, u := range req.Livestreams {
			if u != "" && !contains(uris, u) {
				uris = append(uris, u)
			}
		}
		if len(uris) == 0 {
			errors.WriteHTTPBadRequest(w, "livestream (at:// URI) or livestreams is required", nil)
			return
		}
		items, herr := a.livestreamItems(uris)
		if herr != nil {
			herr.write(w)
			return
		}
		repoDID := items[0].Livestream.RepoDID
		ordered := make([]string, len(items))
		for i, it := range items {
			ordered[i] = it.Livestream.URI
		}
		// endRecords ends the listed records the streamer never stopped,
		// noting each outcome on the response.
		endRecords := func(resp *finalizeLivestreamResponse) {
			for _, it := range items {
				if it.Record.EndedAt != nil {
					resp.Ended = append(resp.Ended, it.Livestream.URI)
					continue
				}
				if err := a.StatefulDB.EndLivestreamRecord(ctx, it.Livestream, it.Record); err != nil {
					log.Error(ctx, "operator finalize: could not end livestream record", "livestream", it.Livestream.URI, "error", err)
					resp.EndErrors = append(resp.EndErrors, it.Livestream.URI+": "+err.Error())
				} else {
					resp.Ended = append(resp.Ended, it.Livestream.URI)
				}
			}
		}
		writeResp := func(resp finalizeLivestreamResponse) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				log.Error(ctx, "error writing response", "error", err)
			}
		}
		if req.EndOnly {
			if !a.StatefulDB.HasUserSession(repoDID) {
				errors.WriteHTTPBadRequest(w, "the streamer has no stored session on this node and the node has no credentials for the account", nil)
				return
			}
			resp := finalizeLivestreamResponse{RepoDID: repoDID, Livestreams: ordered}
			endRecords(&resp)
			log.Log(ctx, "operator finalize: ended livestream records", "livestreams", ordered, "repoDID", repoDID, "ended", resp.Ended, "errors", resp.EndErrors)
			writeResp(resp)
			return
		}
		segs, err := a.StatefulDB.ListS3SegmentsForLivestreams(ctx, ordered)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "list recorded objects", err)
			return
		}
		if len(segs) == 0 {
			errors.WriteHTTPNotFound(w, "no completed recording objects for these livestreams", nil)
			return
		}
		var total int64
		for _, s := range segs {
			total += s.Size
		}
		if !a.StatefulDB.HasUserSession(repoDID) {
			errors.WriteHTTPBadRequest(w, "the streamer has no stored session on this node and the node has no credentials for the account; they must sign in once (or the node be given --account-credentials) before their records can be written", nil)
			return
		}
		publish := req.Publish == nil || *req.Publish
		uu, err := uuid.NewV7()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "uuid", err)
			return
		}
		uploadID := uu.String()
		if err := a.StatefulDB.CreateLivestreamUpload(ctx, uploadID, repoDID, ordered[0]); err != nil {
			errors.WriteHTTPInternalServerError(w, "create upload", err)
			return
		}
		task := statedb.FinalizeLivestreamVODTask{UploadID: uploadID, RepoDID: repoDID, LivestreamURI: ordered[0], LivestreamURIs: ordered}
		video := statedb.VideoDraftForLivestreams(items, req.Title, req.Description)
		if publish {
			task.Publish = video
		}
		if _, err := a.StatefulDB.EnqueueTask(ctx, statedb.TaskFinalizeLivestreamVOD, task, statedb.WithTaskKey("finalize-vod:"+uploadID)); err != nil {
			errors.WriteHTTPInternalServerError(w, "enqueue finalize task", err)
			return
		}
		log.Log(ctx, "operator finalize: livestream VOD queued", "livestreams", ordered, "repoDID", repoDID, "uploadId", uploadID, "objects", len(segs), "bytes", total, "publish", publish)
		resp := finalizeLivestreamResponse{UploadID: uploadID, RepoDID: repoDID, Livestreams: ordered, Objects: len(segs), Bytes: total, Publish: publish, Title: video.Title, TaskQueue: statedb.TaskFinalizeLivestreamVOD}
		if req.EndLivestream {
			endRecords(&resp)
		} else {
			for _, it := range items {
				if it.Record.EndedAt != nil {
					resp.Ended = append(resp.Ended, it.Livestream.URI)
				}
			}
		}
		writeResp(resp)
	}
}

// httpError is a handler failure with the status it should be reported as.
type httpError struct {
	status int
	msg    string
	err    error
}

func (e *httpError) write(w http.ResponseWriter) {
	switch e.status {
	case http.StatusBadRequest:
		errors.WriteHTTPBadRequest(w, e.msg, e.err)
	case http.StatusNotFound:
		errors.WriteHTTPNotFound(w, e.msg, e.err)
	default:
		errors.WriteHTTPInternalServerError(w, e.msg, e.err)
	}
}

// livestreamItems resolves livestream URIs to their indexed rows and decoded
// records, checks they belong to one streamer, and orders them by creation:
// recording order is record order, whatever order the caller listed them in.
func (a *StreamplaceAPI) livestreamItems(uris []string) ([]statedb.LivestreamItem, *httpError) {
	items := make([]statedb.LivestreamItem, 0, len(uris))
	for _, u := range uris {
		ls, err := a.Model.GetLivestream(u)
		if err != nil {
			return nil, &httpError{http.StatusInternalServerError, "get livestream " + u, err}
		}
		if ls == nil {
			return nil, &httpError{http.StatusNotFound, "livestream not indexed on this node: " + u, nil}
		}
		view, err := ls.ToLivestreamView()
		if err != nil {
			return nil, &httpError{http.StatusInternalServerError, "decode livestream " + u, err}
		}
		rec, ok := view.Record.Val.(*placestream.Livestream)
		if !ok {
			return nil, &httpError{http.StatusInternalServerError, "record is not a place.stream.livestream: " + u, nil}
		}
		items = append(items, statedb.LivestreamItem{Livestream: ls, Record: rec})
	}
	for _, it := range items[1:] {
		if it.Livestream.RepoDID != items[0].Livestream.RepoDID {
			return nil, &httpError{http.StatusBadRequest, "the livestreams belong to different streamers", nil}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Record.CreatedAt < items[j].Record.CreatedAt })
	return items, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
