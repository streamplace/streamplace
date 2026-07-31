package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// reindexOriginsPageSize is how many origin records we pull per repo page.
// ServerRepoListRecords takes the server repo lock and decodes each record
// body, so we page rather than asking for everything at once.
const reindexOriginsPageSize = 100

// reindexOriginsResponse reports what a reindex pass did. Scanned counts the
// records walked in the server repo; Indexed counts the rows written. They
// differ only when a record fails to decode or upsert, which is what Errors
// enumerates.
type reindexOriginsResponse struct {
	ServerDID string   `json:"serverDid"`
	Scanned   int      `json:"scanned"`
	Indexed   int      `json:"indexed"`
	Errors    []string `json:"errors,omitempty"`
}

// HandleReindexOrigins rebuilds the local place.stream.media.origin index from
// this node's own server repo.
//
// The server repo is the authority on what we host, and it is always complete:
// publishOrigin commits there synchronously. The local index is the derived,
// lossy copy — it is only ever written by the firehose sync path, so any event
// that connection drops is gone for good, and getVideoList then hides a video
// that the node can in fact serve. (On a --secure node the self-subscription
// could not dial its own listener at all, so nothing published this way was
// ever indexed.) This walks the authority and replays it into the index.
//
// Idempotent, and cheap in the ways that matter: it writes no repo commits and
// emits no firehose events, so it can be run repeatedly and on any node without
// federating a burst of churn. Safe to run while the node is live — every write
// is the same upsert the firehose would have done.
func (a *StreamplaceAPI) HandleReindexOrigins(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqCtx := r.Context()
		serverDID := a.CLI.ServerDID()
		res := reindexOriginsResponse{ServerDID: serverDID}

		cursor := ""
		for {
			page, err := atproto.ServerRepoListRecords(
				reqCtx, constants.PLACE_STREAM_MEDIA_ORIGIN, cursor,
				reindexOriginsPageSize, serverDID, nil,
			)
			if err != nil {
				errors.WriteHTTPInternalServerError(w, "list server repo origins", err)
				return
			}
			for _, rec := range page.Records {
				res.Scanned++
				origin, ok := rec.Value.Val.(*placestream.MediaOrigin)
				if !ok {
					res.Errors = append(res.Errors, rec.Uri+": not a media.origin record")
					continue
				}
				// Index under the blob the record names rather than the rkey.
				// They agree by convention, but the record is the data and the
				// rkey is only a naming convention, so trust the record.
				if err := a.Model.UpsertOwnMediaOrigin(
					reqCtx, serverDID, origin.Blob, origin.Size, origin.MimeType,
				); err != nil {
					log.Error(reqCtx, "reindex origins: upsert failed", "uri", rec.Uri, "error", err)
					res.Errors = append(res.Errors, rec.Uri+": "+err.Error())
					continue
				}
				res.Indexed++
			}
			if page.Cursor == nil || *page.Cursor == "" {
				break
			}
			cursor = *page.Cursor
		}

		log.Log(reqCtx, "reindexed media origins from server repo",
			"serverDid", serverDID, "scanned", res.Scanned, "indexed", res.Indexed, "errors", len(res.Errors))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Error(reqCtx, "error writing reindex-origins response", "error", err)
		}
	}
}
