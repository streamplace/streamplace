package atproto

import (
	"context"
	"fmt"

	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// MediaOriginReindexResult reports the records read from the server repo and
// written to the local index. Errors contains individual record/upsert failures.
type MediaOriginReindexResult struct {
	ServerDID string   `json:"serverDid"`
	Scanned   int      `json:"scanned"`
	Indexed   int      `json:"indexed"`
	Errors    []string `json:"errors,omitempty"`
}

// ReindexOwnMediaOrigins repairs the derived hosting index from this node's
// authoritative server repo. Run before serving listings, since old origins
// may predate direct local indexing or have missed their firehose round-trip.
// It writes no repo commits and leaves other nodes' origin rows untouched.
func ReindexOwnMediaOrigins(ctx context.Context, mod model.Model, serverDID string) (MediaOriginReindexResult, error) {
	res := MediaOriginReindexResult{ServerDID: serverDID}
	cursor := ""
	for {
		page, err := ServerRepoListRecords(ctx, constants.PLACE_STREAM_MEDIA_ORIGIN, cursor, 100, serverDID, nil)
		if err != nil {
			return res, fmt.Errorf("list server repo origins: %w", err)
		}
		for _, rec := range page.Records {
			res.Scanned++
			origin, ok := rec.Value.Val.(*placestream.MediaOrigin)
			if !ok {
				res.Errors = append(res.Errors, rec.Uri+": not a media.origin record")
				continue
			}
			// The record's blob is authoritative; its rkey is only a convention.
			if err := mod.UpsertOwnMediaOrigin(ctx, serverDID, origin.Blob, origin.Size, origin.MimeType); err != nil {
				res.Errors = append(res.Errors, rec.Uri+": "+err.Error())
				continue
			}
			res.Indexed++
		}
		if page.Cursor == nil || *page.Cursor == "" {
			return res, nil
		}
		cursor = *page.Cursor
	}
}
