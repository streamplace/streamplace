package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"
	sperrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// The operator's video CRUD on the internal API (loopback only): the
// counterpart of /finalize-livestream for everything after the finalize.
//
//	GET    /videos?repo=<did>   the repo's video records and its uploads
//	POST   /videos              publish a video record for a finished upload
//	PUT    /videos              retitle / redescribe / retag a record, or point
//	                            it at an upload's tracks (repair or re-finalize)
//	DELETE /videos              delete a record (and, on request, its tracks)
//
// Every write is made with the streamer's stored session, as the finalize
// is; the content blob and the upload row are never touched, so a deleted
// record can be published again from the same upload.

type videoListItem struct {
	URI         string   `json:"uri"`
	CID         string   `json:"cid"`
	Title       string   `json:"title"`
	DurationMs  int64    `json:"durationMs"`
	CreatedAt   string   `json:"createdAt"`
	Tracks      []string `json:"tracks"`
	Connections []string `json:"connections,omitempty"`
	Thumb       bool     `json:"thumb"`
}

type uploadListItem struct {
	UploadID   string    `json:"uploadId"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Backend    string    `json:"backend"`
	Location   string    `json:"location"`
	ContentCID string    `json:"contentCid,omitempty"`
	DurationMs int64     `json:"durationMs"`
	BlobSize   int64     `json:"blobSize"`
	Tracks     int       `json:"tracks"`
	CreatedAt  time.Time `json:"createdAt"`
}

type videoListResponse struct {
	RepoDID string           `json:"repoDID"`
	Videos  []videoListItem  `json:"videos"`
	Uploads []uploadListItem `json:"uploads"`
}

// HandleListVideos (GET /videos?repo=<did>) lists a streamer's video records
// (from their PDS) and this node's uploads for them: the ids the other
// routes take.
func (a *StreamplaceAPI) HandleListVideos(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		did := strings.TrimSpace(r.URL.Query().Get("repo"))
		if _, err := syntax.ParseDID(did); err != nil {
			sperrors.WriteHTTPBadRequest(w, "repo must be a DID", err)
			return
		}
		recs, err := vod.ListVideoRecords(ctx, a.StatefulDB, did)
		if err != nil {
			sperrors.WriteHTTPInternalServerError(w, "list video records", err)
			return
		}
		uploads, err := a.StatefulDB.ListUploadsForRepo(ctx, did)
		if err != nil {
			sperrors.WriteHTTPInternalServerError(w, "list uploads", err)
			return
		}
		resp := videoListResponse{RepoDID: did, Videos: []videoListItem{}, Uploads: []uploadListItem{}}
		for _, rec := range recs {
			item := videoListItem{URI: rec.URI, CID: rec.CID, Tracks: rec.TrackURIs()}
			if item.Tracks == nil {
				item.Tracks = []string{}
			}
			if rec.Video != nil {
				item.Title = rec.Video.Title
				item.DurationMs = rec.Video.DurationMs
				item.CreatedAt = rec.Video.CreatedAt
				item.Thumb = rec.Video.Thumb != nil
				for _, c := range rec.Video.Connections {
					if c.Video_Connection != nil && c.Video_Connection.Ref != nil {
						item.Connections = append(item.Connections, c.Video_Connection.Ref.Uri)
					}
				}
			}
			resp.Videos = append(resp.Videos, item)
		}
		for _, u := range uploads {
			item := uploadListItem{UploadID: u.ID, Status: u.ProcessingStatus, Error: u.ProcessingError, Backend: u.Backend,
				Location: u.Location, ContentCID: u.ContentCID, DurationMs: u.DurationMS, BlobSize: u.BlobSize, CreatedAt: u.CreatedAt}
			if u.TrackURIs != "" {
				var refs []struct{ URI string }
				if json.Unmarshal([]byte(u.TrackURIs), &refs) == nil {
					item.Tracks = len(refs)
				}
			}
			resp.Uploads = append(resp.Uploads, item)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type publishVideoRequest struct {
	// UploadID is a finished upload of the streamer's (a finalized
	// livestream, or any processed upload).
	UploadID string `json:"uploadId"`
	// Title and Description of the record. Title falls back to the first
	// connected livestream's title, then "Livestream".
	Title       string `json:"title"`
	Description string `json:"description"`
	// Livestreams are the livestream records to connect the video to (the
	// records whose recording it is). When empty and the upload came from
	// a livestream, that livestream.
	Livestreams []string `json:"livestreams"`
}

// HandlePublishVideo (POST /videos) publishes a place.stream.video record on
// the streamer's channel for a finished upload: what /finalize-livestream
// does after the finalize, on its own, for an upload finalized with
// publish=false, a publish that failed, or a record that was deleted.
func (a *StreamplaceAPI) HandlePublishVideo(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req publishVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sperrors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		if req.UploadID == "" {
			sperrors.WriteHTTPBadRequest(w, "uploadId is required", nil)
			return
		}
		upload, err := a.StatefulDB.GetUpload(ctx, req.UploadID)
		if err != nil {
			sperrors.WriteHTTPInternalServerError(w, "get upload", err)
			return
		}
		if upload == nil {
			sperrors.WriteHTTPNotFound(w, "no such upload", nil)
			return
		}
		if upload.ProcessingStatus != "done" {
			sperrors.WriteHTTPBadRequest(w, "upload is not finished processing (status "+upload.ProcessingStatus+")", nil)
			return
		}
		uris := req.Livestreams
		if len(uris) == 0 && strings.HasPrefix(upload.Location, "at://") {
			uris = []string{upload.Location}
		}
		var items []livestreamItem
		if len(uris) > 0 {
			var herr *httpError
			items, herr = a.livestreamItems(uris)
			if herr != nil {
				herr.write(w)
				return
			}
			if items[0].ls.RepoDID != upload.RepoDID {
				sperrors.WriteHTTPBadRequest(w, "the livestreams belong to a different streamer than the upload", nil)
				return
			}
		}
		var draft *statedb.VideoDraft
		if len(items) > 0 {
			draft = videoDraftForLivestreams(items, req.Title, req.Description)
		} else {
			title := strings.TrimSpace(req.Title)
			if title == "" {
				title = "Video"
			}
			draft = &statedb.VideoDraft{Title: title}
			if d := strings.TrimSpace(req.Description); d != "" {
				draft.Description = &d
			}
		}
		uri, cid, err := vod.PublishVideo(ctx, a.StatefulDB, a.PlaybackStore, upload.RepoDID, upload.ID, draft.Record())
		if err != nil {
			sperrors.WriteHTTPInternalServerError(w, "publish video", err)
			return
		}
		log.Log(ctx, "operator publish: video record published", "uri", uri, "cid", cid, "uploadId", upload.ID)
		writeJSON(w, http.StatusOK, map[string]any{"uri": uri, "cid": cid, "uploadId": upload.ID, "title": draft.Title})
	}
}

type updateVideoRequest struct {
	URI string `json:"uri"`
	// Absent fields are left alone; "" clears description, [] clears tags.
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Tags        []string `json:"tags"`
	// UploadID makes the upload's tracks the record's source (publishing
	// them if the upload has none yet) and its duration the record's.
	UploadID string `json:"uploadId"`
}

// HandleUpdateVideo (PUT /videos) rewrites a video record in place.
func (a *StreamplaceAPI) HandleUpdateVideo(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req updateVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sperrors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		if req.URI == "" {
			sperrors.WriteHTTPBadRequest(w, "uri is required", nil)
			return
		}
		cid, err := vod.UpdateVideo(ctx, a.StatefulDB, a.PlaybackStore, req.URI, vod.VideoUpdate{
			Title: req.Title, Description: req.Description, Tags: req.Tags, UploadID: req.UploadID,
		})
		if err != nil {
			writeVideoError(w, "update video", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"uri": req.URI, "cid": cid})
	}
}

type deleteVideoRequest struct {
	URI string `json:"uri"`
	// Tracks also deletes the track records the video's source points at.
	Tracks bool `json:"tracks"`
}

// HandleDeleteVideo (DELETE /videos) deletes a video record and, on request,
// its track records. The content blob and the upload stay.
func (a *StreamplaceAPI) HandleDeleteVideo(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var req deleteVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sperrors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		if req.URI == "" {
			sperrors.WriteHTTPBadRequest(w, "uri is required", nil)
			return
		}
		deleted, err := vod.DeleteVideo(ctx, a.StatefulDB, req.URI, req.Tracks)
		if err != nil {
			writeVideoError(w, "delete video", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
	}
}

func writeVideoError(w http.ResponseWriter, msg string, err error) {
	switch {
	case errors.Is(err, vod.ErrNotAVideo):
		sperrors.WriteHTTPBadRequest(w, msg, err)
	case errors.Is(err, vod.ErrUploadNotFound):
		sperrors.WriteHTTPNotFound(w, msg+": no such upload for this streamer", err)
	case errors.Is(err, vod.ErrUploadNotReady):
		sperrors.WriteHTTPBadRequest(w, msg+": upload is not finished processing", err)
	default:
		sperrors.WriteHTTPInternalServerError(w, msg, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
