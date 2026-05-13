package vod

import (
	"bytes"
	"context"
	"fmt"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
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
	mod      model.Model
	in       Input
	cid      string
	size     int64
	mimeType string
	probe    media.VODResult
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

	if err := publishOrigin(ctx, p.cli, p.mod, p.cid, p.size, p.mimeType); err != nil {
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
		ref, err := publishTrack(ctx, client, p.in.RepoDID, p.cid, "1", "video", p.probe.Video, nil)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "video_track")
			return fmt.Errorf("publish video track: %w", err)
		}
		sourceTracks = append(sourceTracks, ref)
	}
	if p.probe.Audio != nil {
		ref, err := publishTrack(ctx, client, p.in.RepoDID, p.cid, "2", "audio", nil, p.probe.Audio)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "audio_track")
			return fmt.Errorf("publish audio track: %w", err)
		}
		sourceTracks = append(sourceTracks, ref)
	}

	if err := publishVideo(ctx, client, p.in, p.probe, sourceTracks); err != nil {
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
// After committing to the server repo we also upsert the equivalent
// row in the local index. Server-repo commits don't flow through the
// bluesky firehose (it's our own repo, not a federated PDS), so
// without the synthetic upsert here the playback path on the same
// node wouldn't know about its own published blobs. Future federation
// receivers (subscribing to other nodes' server-repo firehoses) will
// reach the same model row via the place.stream.media.origin case in
// pkg/atproto/sync.go.
func publishOrigin(ctx context.Context, cli *config.CLI, mod model.Model, cid string, size int64, mimeType string) error {
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

	serverDID := cli.ServerDID()
	uri := fmt.Sprintf("at://%s/%s/%s", serverDID, constants.PLACE_STREAM_MEDIA_ORIGIN, cid)
	// Re-encode for the index; we don't have the record CID from the
	// commit (CommitServerRepoRecord swallows it) so we use the blob
	// CID as a stand-in. Downstream callers care about the blob, not
	// the record CID.
	recBytes := bytes.Buffer{}
	if err := rec.MarshalCBOR(&recBytes); err != nil {
		span.RecordError(err)
		// Non-fatal: the record is in the repo; we just can't index it
		// locally right now. The firehose path will resync if/when it
		// comes online.
		log.Warn(ctx, "failed to CBOR-encode origin record for index", "error", err)
		return nil
	}
	if err := mod.UpsertMediaOrigin(ctx, &model.MediaOrigin{
		URI:       uri,
		CID:       cid, // record CID not available from server-repo commit; reuse blob CID
		ServerDID: serverDID,
		RKey:      cid,
		Blob:      cid,
		Size:      size,
		MimeType:  mimeType,
		Record:    recBytes.Bytes(),
		IndexedAt: time.Now(),
	}); err != nil {
		span.RecordError(err)
		log.Warn(ctx, "failed to index media.origin locally", "error", err)
	}

	log.Log(ctx, "published media.origin",
		"collection", constants.PLACE_STREAM_MEDIA_ORIGIN,
		"rkey", cid,
		"size", size,
		"uri", uri,
	)
	return nil
}

// publishTrack creates one place.stream.media.track record in the
// user's repo. trackID is the 1-indexed track within the MUXL
// container ("1" for video, "2" for audio in our standard layout).
// Exactly one of videoMeta / audioMeta should be non-nil; the other
// is ignored.
func publishTrack(ctx context.Context, client XRPCClient, did, cid, trackID, mediaType string, videoMeta *media.VODVideoTrack, audioMeta *media.VODAudioTrack) (*comatproto.RepoStrongRef, error) {
	ctx, span := vodTracer.Start(ctx, "vod.publishTrack", trace.WithAttributes(
		attribute.String("cid", cid),
		attribute.String("track_id", trackID),
		attribute.String("media_type", mediaType),
	))
	defer span.End()

	meta := &streamplace.MediaTrack_CommonMetadata{}
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
				TrackId:       trackID,
				MediaType:     mediaType,
			},
		},
		Metadata: meta,
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
// client didn't send one.
func publishVideo(ctx context.Context, client XRPCClient, in Input, probe media.VODResult, tracks []*comatproto.RepoStrongRef) error {
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
		Source: &streamplace.Video_Source{
			MediaDefs_SourceTracks: &streamplace.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        tracks,
			},
		},
	}
	if duration > 0 {
		rec.Duration = &duration
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
