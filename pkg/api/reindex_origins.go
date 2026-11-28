package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/vod"
)

// reindexOriginsPageSize is how many origin records we pull per repo page.
// ServerRepoListRecords takes the server repo lock and decodes each record
// body, so we page rather than asking for everything at once.
const reindexOriginsPageSize = 100

// reindexOriginsResponse reports server-repo records scanned and indexed
// during replay. Recovered counts origins newly committed to the server repo.
// Errors enumerates per-record or per-blob failures in either pass.
type reindexOriginsResponse struct {
	ServerDID string   `json:"serverDid"`
	Scanned   int      `json:"scanned"`
	Indexed   int      `json:"indexed"`
	Recovered int      `json:"recovered"`
	Errors    []string `json:"errors,omitempty"`
}

// HandleReindexOrigins rebuilds the local place.stream.media.origin index from
// this node's own server repo.
//
// This repairs missed self-subscription events without creating commits.
// With recover=true, it also publishes missing origins for indexed videos
// whose playback objects are still present in the configured store.
func (a *StreamplaceAPI) HandleReindexOrigins(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqCtx := r.Context()
		serverDID := a.CLI.ServerDID()
		res := reindexOriginsResponse{ServerDID: serverDID}
		var knownOrigins map[string]struct{}
		if r.URL.Query().Get("recover") == "true" {
			if a.PlaybackStore == nil {
				errors.WriteHTTPInternalServerError(w, "playback store not configured", nil)
				return
			}
			knownOrigins = make(map[string]struct{})
		}

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
				if knownOrigins != nil {
					knownOrigins[origin.Blob] = struct{}{}
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
		if knownOrigins != nil {
			if err := a.recoverStoredOrigins(reqCtx, knownOrigins, &res); err != nil {
				errors.WriteHTTPInternalServerError(w, "recover stored origins", err)
				return
			}
		}

		log.Log(reqCtx, "reindexed media origins from server repo",
			"serverDid", serverDID, "scanned", res.Scanned, "indexed", res.Indexed,
			"recovered", res.Recovered, "errors", len(res.Errors))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Error(reqCtx, "error writing reindex-origins response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) recoverStoredOrigins(ctx context.Context, known map[string]struct{}, res *reindexOriginsResponse) error {
	// List once rather than HEAD every foreign blob in the model index.
	keys, err := a.PlaybackStore.List(ctx, vod.BlobsPrefix)
	if err != nil {
		return fmt.Errorf("list playback objects: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	slices.Sort(keys)
	return a.Model.WalkVideoContentBlobs(ctx, func(cid string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, exists := known[cid]; exists {
			return nil
		}
		mediaKey := vod.BlobsPrefix + cid + ".mp4"
		if _, exists := slices.BinarySearch(keys, mediaKey); !exists {
			return nil
		}
		known[cid] = struct{}{} // Videos and clips can share the same blob.
		_, hasMetafile := slices.BinarySearch(keys, vod.BlobsPrefix+cid+".json")
		if err := a.recoverStoredOrigin(ctx, cid, mediaKey, hasMetafile, res); err != nil {
			log.Error(ctx, "recover origins: blob failed", "cid", cid, "error", err)
			res.Errors = append(res.Errors, cid+": "+err.Error())
		}
		return nil
	})
}

func (a *StreamplaceAPI) recoverStoredOrigin(ctx context.Context, cid, mediaKey string, hasMetafile bool, res *reindexOriginsResponse) error {
	if _, err := bdasl.Parse(cid); err != nil {
		return fmt.Errorf("invalid content CID: %w", err)
	}
	if !hasMetafile {
		return fmt.Errorf("missing playback metafile")
	}
	r, err := a.PlaybackStore.Open(ctx, mediaKey)
	if err != nil {
		return fmt.Errorf("stat playback blob: %w", err)
	}
	size := r.Size()
	if err := r.Close(); err != nil {
		return fmt.Errorf("close playback blob: %w", err)
	}
	if size <= 0 {
		return fmt.Errorf("empty playback blob")
	}
	const mimeType = "video/mp4"
	if err := vod.PublishOrigin(ctx, a.CLI, cid, size, mimeType); err != nil {
		return fmt.Errorf("publish media.origin: %w", err)
	}
	res.Recovered++
	if err := a.Model.UpsertOwnMediaOrigin(ctx, res.ServerDID, cid, size, mimeType); err != nil {
		return fmt.Errorf("index media.origin: %w", err)
	}
	return nil
}
