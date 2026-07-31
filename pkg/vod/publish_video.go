package vod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/comatproto"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
)

// ErrUploadNotFound / ErrUploadNotReady let the publishVideo XRPC handler map
// PublishVideo failures onto the right HTTP status (404 / 409) without
// reaching into this package's internals.
var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrUploadNotReady = errors.New("upload not finished processing")
)

// PublishVideo creates a place.stream.video record in the authenticated
// user's repo for a finished upload, server-side. The caller supplies the
// record it would otherwise putRecord itself (title, description, tags,
// optional thumb, ...); PublishVideo overrides the fields the server is
// authoritative about — `source` (the published track refs) and `durationMs`,
// both from the Upload row — and, if the record carries no thumb, generates
// one from the processed video and attaches it. Returns the new record's
// URI + CID.
//
// This is the engine behind the place.stream.media.publishVideo procedure:
// letting the server write the record is what lets us backfill a thumbnail
// (the client may not supply one) using the same generateThumbnail path
// vod-test exercises.
func PublishVideo(ctx context.Context, state *statedb.StatefulDB, store blob.Store, did, uploadID string, video *placestream.Video) (string, string, error) {
	ctx = log.WithLogValues(ctx, "func", "PublishVideo", "did", did, "uploadId", uploadID)
	ctx, span := vodTracer.Start(ctx, "vod.PublishVideo", trace.WithAttributes(
		attribute.String("did", did),
		attribute.String("upload_id", uploadID),
	))
	defer span.End()

	upload, err := state.GetUpload(ctx, uploadID)
	if err != nil {
		span.RecordError(err)
		return "", "", fmt.Errorf("get upload: %w", err)
	}
	if upload == nil || upload.RepoDID != did {
		return "", "", ErrUploadNotFound
	}
	if upload.ProcessingStatus != "done" {
		return "", "", ErrUploadNotReady
	}

	// Server-authoritative fields: source tracks, duration, and the
	// creation timestamp come from the server at publish time, not from
	// whatever the client put in the record.
	video.LexiconTypeID = constants.PLACE_STREAM_VIDEO
	video.DurationMs = upload.DurationMS
	video.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	tracks, err := sourceTracksFromUpload(upload)
	if err != nil {
		span.RecordError(err)
		return "", "", err
	}
	video.Source = placestream.Video_Source{
		MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
			LexiconTypeID: "place.stream.media.defs#sourceTracks",
			Tracks:        tracks,
		},
	}

	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		span.RecordError(err)
		return "", "", fmt.Errorf("get user xrpc client: %w", err)
	}

	// Backfill a thumbnail when the client didn't supply one. Non-fatal:
	// publish without it on any failure, matching the old inline behavior.
	if video.Thumb == nil && upload.ContentCID != "" {
		thumb, terr := generateAndUploadThumbnail(ctx, client, store, upload.ContentCID)
		if terr != nil {
			log.Warn(ctx, "publishVideo: thumbnail backfill failed; publishing without thumb", "error", terr)
		} else {
			video.Thumb = thumb
		}
	}
	span.SetAttributes(attribute.Bool("thumb_present", video.Thumb != nil))

	rkey := spid.TIDClock.Next().String()
	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_VIDEO,
		Record:     &glex.LexiconTypeDecoder{Val: video},
		Rkey:       rkey,
		Repo:       did,
	}
	out := comatproto.RepoPutRecord_Output{}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "put_record")
		return "", "", fmt.Errorf("putRecord video: %w", err)
	}
	span.SetAttributes(attribute.String("uri", out.Uri), attribute.String("cid", out.Cid))
	log.Log(ctx, "published video record (server-side)",
		"uri", out.Uri, "uploadId", uploadID, "title", video.Title)
	return out.Uri, out.Cid, nil
}

// sourceTracksFromUpload turns the {uri,cid} track refs stored on the Upload
// row (by publishRecords) into the strongRefs for the video record's source.
func sourceTracksFromUpload(upload *statedb.Upload) ([]comatproto.RepoStrongRef, error) {
	if upload.TrackURIs == "" {
		return nil, nil
	}
	var refs []trackRefJSON
	if err := json.Unmarshal([]byte(upload.TrackURIs), &refs); err != nil {
		return nil, fmt.Errorf("decode track refs: %w", err)
	}
	tracks := make([]comatproto.RepoStrongRef, 0, len(refs))
	for _, r := range refs {
		tracks = append(tracks, comatproto.RepoStrongRef{Uri: r.URI, Cid: r.CID})
	}
	return tracks, nil
}

// generateAndUploadThumbnail renders a thumbnail from the processed video
// blob (via the same path as vod-test) and uploads it to the user's PDS,
// returning the resulting blob ref.
func generateAndUploadThumbnail(ctx context.Context, client XRPCClient, store blob.Store, contentCID string) (*glex.Blob, error) {
	metafile, err := readMetafile(ctx, store, contentCID)
	if err != nil {
		return nil, fmt.Errorf("read metafile: %w", err)
	}
	thumb, err := generateThumbnail(ctx, store, contentCID, metafile)
	if err != nil {
		return nil, fmt.Errorf("generate thumbnail: %w", err)
	}
	var uploadOut comatproto.RepoUploadBlob_Output
	if err := client.Do(ctx, xrpc.Procedure, thumbnailMimeType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(thumb), &uploadOut); err != nil {
		return nil, fmt.Errorf("upload thumb blob: %w", err)
	}
	return &uploadOut.Blob, nil
}
