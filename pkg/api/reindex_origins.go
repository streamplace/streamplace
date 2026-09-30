package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
)

// HandleReindexOrigins rebuilds the local place.stream.media.origin index from
// this node's own server repo.
//
// The server repo is authoritative; the local hosting index may be incomplete
// after a missed firehose event. Startup runs the same reconciliation before
// serving listings. This endpoint also allows repair while the node is live.
//
// Idempotent, and cheap in the ways that matter: it writes no repo commits and
// emits no firehose events, so it can be run repeatedly and on any node without
// federating a burst of churn. Safe to run while the node is live — every write
// is the same upsert the firehose would have done.
func (a *StreamplaceAPI) HandleReindexOrigins(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqCtx := r.Context()
		res, err := atproto.ReindexOwnMediaOrigins(reqCtx, a.Model, a.CLI.ServerDID())
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "reindex origins", err)
			return
		}
		for _, failure := range res.Errors {
			log.Error(reqCtx, "reindex origins: record failed", "error", failure)
		}

		log.Log(reqCtx, "reindexed media origins from server repo",
			"serverDid", res.ServerDID, "scanned", res.Scanned, "indexed", res.Indexed, "errors", len(res.Errors))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Error(reqCtx, "error writing reindex-origins response", "error", err)
		}
	}
}
