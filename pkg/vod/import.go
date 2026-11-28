package vod

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// ImportFile runs a local media file through the VOD pipeline into store, with no
// upload row or statedb: the content blob and metafile land at their
// content-addressed keys. The caller publishes records from the result.
func ImportFile(ctx context.Context, store blob.Store, path string) (*ImportResult, error) {
	ctx, span := vodTracer.Start(ctx, "vod.ImportFile")
	defer span.End()

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	span.SetAttributes(attribute.Int64("source_size_bytes", st.Size()))

	stagingKey := StagingPrefix + "import-" + uuid.NewString() + ".mp4"
	res, err := buildContentBlob(ctx, span, store, f, st.Size(), stagingKey, &countingWriter{}, func(int) {})
	if err != nil {
		return nil, err
	}
	log.Log(ctx, "imported VOD file", "path", path, "cid", res.CID, "blob_size", res.BlobSize)
	return res, nil
}

// PublishImportTracks publishes the place.stream.media.track records for an
// imported file into did's repo and returns them as a video record's source.
func PublishImportTracks(ctx context.Context, client XRPCClient, did string, res *ImportResult) (*placestream.MediaDefs_SourceTracks, error) {
	source, err := publishSourceTracks(ctx, client, did, res.CID, res.BlobSize, res.SigningKey, res.Probe)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("imported file %s has no audio or video tracks", res.CID)
	}
	return source, nil
}
