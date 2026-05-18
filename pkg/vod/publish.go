package vod

import (
	"bytes"
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

// XRPCClient is the subset of indigo's xrpc.Client we actually call.
// Pulled out as an interface so tests / dev wrappers can substitute.
// Mirrors pkg/director's XRPCClient for consistency.
type XRPCClient interface {
	Do(ctx context.Context, method string, contentType string, path string, queryParams map[string]any, body any, out any) error
}

// publishParams bundles everything needed to publish the four records
// (one origin + two tracks + one video) that describe a processed VOD.
type publishParams struct {
	cli      *config.CLI
	state    *statedb.StatefulDB
	in       Input
	cid      string
	size     int64
	mimeType string
	probe    media.VODResult
	// signingKey is the did:key of the ephemeral key that C2PA-signed
	// this upload's segments. Recorded on every track record.
	signingKey string
	// thumbnail is a JPEG generated from ~halfway through the video, or
	// nil if generation failed. Uploaded to the user's PDS and attached
	// to the place.stream.video record when present.
	thumbnail []byte
}

// publishRecords does the post-processing record publish:
//
//  1. place.stream.media.origin in the SERVER's repo (we attest that
//     this blob is fetchable from us). Idempotent: rkey is the CID.
//  2. place.stream.media.track in the USER's repo, one per A/V track,
//     via the user's stored OAuth session.
//  3. place.stream.video in the USER's repo, source = sourceTracks
//     referencing the strongRefs returned by step 2.
//
// Errors are surfaced to the caller; the calling task processor's
// retry behavior will then re-run the whole pipeline. The origin
// record is idempotent on retry; the track + video records are not
// yet (a retry would produce duplicates). We accept that for V1; a
// follow-up will track created rkeys on the Upload row.
func publishRecords(ctx context.Context, p publishParams) error {
	ctx, span := vodTracer.Start(ctx, "vod.publishRecords", trace.WithAttributes(
		attribute.String("cid", p.cid),
		attribute.String("did", p.in.RepoDID),
	))
	defer span.End()

	if err := publishOrigin(ctx, p.cli, p.cid, p.size, p.mimeType); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "origin")
		return fmt.Errorf("publish origin: %w", err)
	}

	client, err := getUserXRPCClient(ctx, p.state, p.in.RepoDID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get_client")
		return fmt.Errorf("get user xrpc client: %w", err)
	}

	var sourceTracks []*comatproto.RepoStrongRef
	if p.probe.Video != nil {
		ref, err := publishTrack(ctx, client, p.in.RepoDID, p.cid, p.size, p.probe.DurationMS, "1", "video", p.signingKey, p.probe.Video, nil)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "video_track")
			return fmt.Errorf("publish video track: %w", err)
		}
		sourceTracks = append(sourceTracks, ref)
	}
	if p.probe.Audio != nil {
		ref, err := publishTrack(ctx, client, p.in.RepoDID, p.cid, p.size, p.probe.DurationMS, "2", "audio", p.signingKey, nil, p.probe.Audio)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "audio_track")
			return fmt.Errorf("publish audio track: %w", err)
		}
		sourceTracks = append(sourceTracks, ref)
	}

	if err := publishVideo(ctx, client, p.in, p.probe, sourceTracks, p.thumbnail); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "video")
		return fmt.Errorf("publish video: %w", err)
	}

	return nil
}

// publishOrigin attests that this server has the blob with the given
// CID. The record's rkey is the CID, so retries of the same processed
// VOD overwrite the existing record rather than accumulating dupes.
//
// Indexing is deliberately NOT done here: server-repo commits federate
// through the same firehose path as everything else, and the
// place.stream.media.origin case in pkg/atproto/sync.go handles the
// upsert into the local model. Keeping publish and index separate
// means this package can run as a standalone microservice without
// dragging the indexer along.
func publishOrigin(ctx context.Context, cli *config.CLI, cid string, size int64, mimeType string) error {
	ctx, span := vodTracer.Start(ctx, "vod.publishOrigin", trace.WithAttributes(
		attribute.String("cid", cid),
		attribute.Int64("size", size),
	))
	defer span.End()

	rec := &streamplace.MediaOrigin{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
		Blob:          cid,
		Size:          size,
		MimeType:      mimeType,
	}
	if err := atproto.CommitServerRepoRecord(ctx, cli, constants.PLACE_STREAM_MEDIA_ORIGIN, cid, rec); err != nil {
		span.RecordError(err)
		return err
	}
	log.Log(ctx, "published media.origin",
		"collection", constants.PLACE_STREAM_MEDIA_ORIGIN,
		"rkey", cid,
		"size", size,
	)
	return nil
}

// publishTrack creates one place.stream.media.track record in the
// user's repo. trackID is the 1-indexed track within the MUXL
// container ("1" for video, "2" for audio in our standard layout).
// blobSize is the byte size of the shared MUXL container blob;
// durationMS is the source duration in milliseconds (same for every
// track of one upload since they all read from the same container).
// signingKey is the did:key whose ephemeral private half C2PA-signed
// this upload's segments — the same key signs every track of an upload.
// Exactly one of videoMeta / audioMeta should be non-nil; the other
// is ignored.
func publishTrack(ctx context.Context, client XRPCClient, did, cid string, blobSize, durationMS int64, trackID, mediaType, signingKey string, videoMeta *media.VODVideoTrack, audioMeta *media.VODAudioTrack) (*comatproto.RepoStrongRef, error) {
	ctx, span := vodTracer.Start(ctx, "vod.publishTrack", trace.WithAttributes(
		attribute.String("cid", cid),
		attribute.String("track_id", trackID),
		attribute.String("media_type", mediaType),
	))
	defer span.End()

	meta := &streamplace.MediaTrack_CommonMetadata{
		LexiconTypeID: "place.stream.media.track#commonMetadata",
		DurationMs:    &durationMS,
	}
	if videoMeta != nil {
		meta.Video = &streamplace.Segment_Video{
			Codec:  "h264",
			Width:  int64(videoMeta.Width),
			Height: int64(videoMeta.Height),
		}
		if videoMeta.FPSDen > 0 {
			meta.Video.Framerate = &streamplace.Segment_Framerate{
				Num: int64(videoMeta.FPSNum),
				Den: int64(videoMeta.FPSDen),
			}
		}
	}
	if audioMeta != nil {
		meta.Audio = &streamplace.Segment_Audio{
			Codec:    audioCodecForLexicon(audioMeta),
			Rate:     int64(audioMeta.Rate),
			Channels: int64(audioMeta.Channels),
		}
	}

	rec := &streamplace.MediaTrack{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_TRACK,
		Track: &streamplace.MediaTrack_Track{
			MediaDefs_MuxlTrack: &streamplace.MediaDefs_MuxlTrack{
				LexiconTypeID: "place.stream.media.defs#muxlTrack",
				Blob:          cid,
				Size:          &blobSize,
				TrackId:       trackID,
				MediaType:     mediaType,
				SigningKey:    &signingKey,
			},
		},
		Metadata: &streamplace.MediaTrack_Metadata{
			MediaTrack_CommonMetadata: meta,
		},
	}

	rkey := spid.TIDClock.Next().String()
	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_MEDIA_TRACK,
		Record:     &lexutil.LexiconTypeDecoder{Val: rec},
		Rkey:       rkey,
		Repo:       did,
	}
	out := comatproto.RepoPutRecord_Output{}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("putRecord track %s: %w", mediaType, err)
	}
	span.SetAttributes(
		attribute.String("uri", out.Uri),
		attribute.String("cid", out.Cid),
	)
	log.Log(ctx, "published media.track",
		"mediaType", mediaType,
		"trackId", trackID,
		"uri", out.Uri,
	)
	return &comatproto.RepoStrongRef{
		LexiconTypeID: "com.atproto.repo.strongRef",
		Uri:           out.Uri,
		Cid:           out.Cid,
	}, nil
}

// publishVideo creates the top-level place.stream.video record in the
// user's repo, referencing the track records via sourceTracks. The
// title defaults to the upload's filename hint, or "Untitled" if the
// client didn't send one. When thumbnail bytes are supplied they're
// uploaded to the user's PDS and attached as the record's thumb blob;
// an upload failure is logged but doesn't fail the publish.
func publishVideo(ctx context.Context, client XRPCClient, in Input, probe media.VODResult, tracks []*comatproto.RepoStrongRef, thumbnail []byte) error {
	ctx, span := vodTracer.Start(ctx, "vod.publishVideo", trace.WithAttributes(
		attribute.Int("track_count", len(tracks)),
		attribute.Int64("duration_ms", probe.DurationMS),
	))
	defer span.End()

	title := in.Filename
	if title == "" {
		title = "Untitled"
	}
	duration := probe.DurationMS

	rec := &streamplace.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         title,
		DurationMs:    duration,
		Source: &streamplace.Video_Source{
			MediaDefs_SourceTracks: &streamplace.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        tracks,
			},
		},
	}

	if len(thumbnail) > 0 {
		var uploadOut comatproto.RepoUploadBlob_Output
		if err := client.Do(ctx, xrpc.Procedure, thumbnailMimeType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(thumbnail), &uploadOut); err != nil {
			log.Warn(ctx, "failed to upload VOD thumbnail blob; publishing without thumbnail", "error", err)
		} else {
			rec.Thumb = uploadOut.Blob
			span.SetAttributes(attribute.Bool("thumb_uploaded", true))
		}
	}

	rkey := spid.TIDClock.Next().String()
	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_VIDEO,
		Record:     &lexutil.LexiconTypeDecoder{Val: rec},
		Rkey:       rkey,
		Repo:       in.RepoDID,
	}
	out := comatproto.RepoPutRecord_Output{}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		span.RecordError(err)
		return fmt.Errorf("putRecord video: %w", err)
	}
	span.SetAttributes(
		attribute.String("uri", out.Uri),
		attribute.String("cid", out.Cid),
	)
	log.Log(ctx, "published video record",
		"title", title,
		"uri", out.Uri,
		"duration_ms", duration,
	)
	return nil
}

// getUserXRPCClient resolves the user's stored OAuth session, refreshes
// it if expiring, and returns an authenticated xrpc client targeting
// the user's PDS. Mirrors the pattern used by finalize_livestream and
// stream_session.
func getUserXRPCClient(ctx context.Context, state *statedb.StatefulDB, did string) (XRPCClient, error) {
	session, err := state.GetSessionByDID(did)
	if err != nil {
		return nil, fmt.Errorf("get oauth session for %s: %w", did, err)
	}
	if session == nil {
		return nil, fmt.Errorf("no oauth session for %s", did)
	}
	session, err = state.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		return nil, fmt.Errorf("refresh oauth session for %s: %w", did, err)
	}
	client, err := state.OATProxy.GetXrpcClient(session)
	if err != nil {
		return nil, fmt.Errorf("get xrpc client for %s: %w", did, err)
	}
	return client, nil
}

// audioCodecForLexicon maps a parsebin caps name to one of the values
// allowed by the place.stream.segment#audio.codec enum
// ({"opus", "aac"}). When the chain transcoded the input audio to AAC,
// the probe reports codec="mpeg" with MPEGVersion=4 — that's AAC.
func audioCodecForLexicon(a *media.VODAudioTrack) string {
	switch a.Codec {
	case "x-opus":
		return "opus"
	case "mpeg":
		// MPEG-4 AAC (and MPEG-2 AAC) are both AAC for our purposes.
		return "aac"
	default:
		// Fall through to AAC; we transcoded everything else to AAC.
		return "aac"
	}
}
