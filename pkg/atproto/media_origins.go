package atproto

import (
	"context"
	"errors"
	"fmt"
	"strings"

	atrepo "github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
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
// A single immutable repo snapshot avoids walking and sorting the full origin
// collection once per page. Only opening the snapshot holds the writer lock;
// record reads and index writes do not block new server-repo commits.
func ReindexOwnMediaOrigins(ctx context.Context, mod model.Model, serverDID string) (MediaOriginReindexResult, error) {
	res := MediaOriginReindexResult{ServerDID: serverDID}
	serverRepoLock.Lock()
	r, ses, err := OpenServerRepo(ctx)
	serverRepoLock.Unlock()
	if err != nil {
		return res, fmt.Errorf("open server repo origins: %w", err)
	}

	prefix := constants.PLACE_STREAM_MEDIA_ORIGIN + "/"
	err = r.ForEach(ctx, prefix, func(path string, recordCID cid.Cid) error {
		if !strings.HasPrefix(path, prefix) {
			return atrepo.ErrDoneIterating
		}
		raw, err := getBlock(ctx, ses, recordCID)
		if err != nil {
			return fmt.Errorf("read origin at://%s/%s: %w", serverDID, path, err)
		}
		rec, err := glex.CborDecodeValue(raw)
		if err != nil {
			return fmt.Errorf("decode origin at://%s/%s: %w", serverDID, path, err)
		}
		res.Scanned++
		origin, ok := rec.(*placestream.MediaOrigin)
		if !ok {
			res.Errors = append(res.Errors, "at://"+serverDID+"/"+path+": not a media.origin record")
			return nil
		}
		// The record's blob is authoritative; its rkey is only a convention.
		if err := mod.UpsertOwnMediaOrigin(ctx, serverDID, origin.Blob, origin.Size, origin.MimeType); err != nil {
			res.Errors = append(res.Errors, "at://"+serverDID+"/"+path+": "+err.Error())
			return nil
		}
		res.Indexed++
		return nil
	})
	if err != nil && !errors.Is(err, atrepo.ErrDoneIterating) {
		return res, fmt.Errorf("walk server repo origins: %w", err)
	}
	return res, nil
}
