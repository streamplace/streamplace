package vod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	glexrt "github.com/streamplace/glex/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/comatproto"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// ErrDraftNotFound / ErrDraftNotReady let the publishDraft XRPC handler map
// PublishDraft failures onto the right HTTP status (404 / 409) without
// reaching into this package's internals.
var (
	ErrDraftNotFound = errors.New("draft not found")
	ErrDraftNotReady = errors.New("draft is not ready to publish")
)

// PublishDraft promotes a ready draft VOD to a public place.stream.video
// record in the author's repo, then deletes the draft. It mirrors
// PublishVideo's tail: the draft's editable fields (title, description, tags,
// activity, connections, content warnings/rights, thumb) plus its
// server-authoritative fields (source, durationMs) are carried over into the
// video record, createdAt is set server-side, and a thumbnail is backfilled
// from the content blob (via the draft row's ContentCID) when the draft
// carries none. After a successful putRecord the draft row is deleted.
//
// did is the authenticated user; draftURI is the ats:// URI of their draft.
func PublishDraft(ctx context.Context, state *statedb.StatefulDB, store blob.Store, did, draftURI string) (string, string, error) {
	ctx, span := vodTracer.Start(ctx, "vod.PublishDraft", trace.WithAttributes(
		attribute.String("did", did),
		attribute.String("draft_uri", draftURI),
	))
	defer span.End()

	dv, err := state.GetDraft(ctx, draftURI)
	if err != nil {
		span.RecordError(err)
		return "", "", fmt.Errorf("get draft: %w", err)
	}
	if dv == nil || dv.UserDID != did {
		return "", "", ErrDraftNotFound
	}
	rec, err := unmarshalDraftRecord(dv.Data)
	if err != nil {
		span.RecordError(err)
		return "", "", err
	}
	if rec.Status != "ready" {
		span.SetStatus(codes.Error, "not ready")
		return "", "", fmt.Errorf("%w: status is %q", ErrDraftNotReady, rec.Status)
	}

	// Build the public video record from the draft. The draft's editable
	// fields map 1:1; source/durationMs come straight from the CBOR body (the
	// server filled them at SetDraftReady time). createdAt is set here, as
	// PublishVideo does.
	video := &placestream.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         rec.Title,
		Description:   rec.Description,
		DurationMs:    derefInt64(rec.DurationMs),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	video.DescriptionFacets = rec.DescriptionFacets
	video.Tags = rec.Tags
	video.Connections = draftConnectionsToVideo(rec.Connections)
	video.Activity = draftActivityToVideo(rec.Activity)
	video.ContentWarnings = rec.ContentWarnings
	video.ContentRights = rec.ContentRights
	video.Thumb = rec.Thumb

	// Publish the place.stream.media.track records now (deferred from
	// processing time so they don't leak before the video record). Read the
	// probe + signing key from the tied Upload row, publish one track per
	// A/V stream, and build the video's source from the fresh strongRefs.
	// If there's no Upload row (e.g. a draft whose upload predates this
	// change, or a draft created outside the upload flow), fall back to
	// carrying over any source the draft already carries.
	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get_client")
		return "", "", fmt.Errorf("get user xrpc client: %w", err)
	}
	sourceTracks, terr := publishTracksFromUpload(ctx, state, client, did, dv)
	if terr != nil {
		span.RecordError(terr)
		span.SetStatus(codes.Error, "publish_tracks")
		return "", "", fmt.Errorf("publish tracks: %w", terr)
	}
	if sourceTracks != nil {
		video.Source = placestream.Video_Source{
			MediaDefs_SourceTracks: sourceTracks,
		}
	} else if rec.Source != nil && rec.Source.MediaDefs_SourceTracks != nil {
		// Legacy fallback: a draft that already carries source (e.g. published
		// via the old at-ready-time path). Carry it over.
		video.Source = placestream.Video_Source{
			MediaDefs_SourceTracks: rec.Source.MediaDefs_SourceTracks,
		}
	} else if rec.Source != nil && rec.Source.MediaDefs_SourceClip != nil {
		video.Source = placestream.Video_Source{
			MediaDefs_SourceClip: rec.Source.MediaDefs_SourceClip,
		}
	}

	// Reuse the draft's tid as the published video's rkey. The draft is
	// deleted after a successful publish, so there's no collision, and sharing
	// the tid lets a single /upload/video/<tid> route render either a draft
	// (ats://) or its published VOD (at://) by the same key.
	rkey, err := draftTID(draftURI)
	if err != nil {
		span.RecordError(err)
		return "", "", err
	}

	// Backfill a thumbnail when the draft didn't carry one, using the content
	// blob (MUXL CID on the draft row). Non-fatal: publish without it on any
	// failure, matching PublishVideo's behavior.
	if video.Thumb == nil && dv.ContentCID != "" {
		thumb, terr := generateAndUploadThumbnail(ctx, client, store, dv.ContentCID)
		if terr != nil {
			log.Warn(ctx, "publishDraft: thumbnail backfill failed; publishing without thumb", "error", terr)
		} else {
			video.Thumb = thumb
		}
	}
	span.SetAttributes(attribute.Bool("thumb_present", video.Thumb != nil))

	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_VIDEO,
		Record:     &glexrt.LexiconTypeDecoder{Val: video},
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
	log.Log(ctx, "published video record from draft",
		"uri", out.Uri, "draftUri", draftURI, "title", video.Title)

	// Draft fulfilled — delete it now that the public record exists.
	if _, err := state.DeleteDraft(ctx, draftURI); err != nil {
		// The video was published successfully; a delete failure here leaves a
		// stale draft the user can just discard. Log and proceed.
		log.Warn(ctx, "publishDraft: failed to delete draft after publish", "draftUri", draftURI, "error", err)
	}

	return out.Uri, out.Cid, nil
}

// unmarshalDraftRecord CBOR-decodes a draft record body. (Sibling of
// statedb.unmarshalDraft, exposed here so PublishDraft stays in pkg/vod.)
func unmarshalDraftRecord(data []byte) (*placestream.VodDraftVideo, error) {
	var rec placestream.VodDraftVideo
	if err := rec.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("unmarshal draft CBOR: %w", err)
	}
	return &rec, nil
}

// draftTID extracts the record key (tid) from a draft's ats:// URI. The URI is
// ats://{did}/place.stream.vod.drafts/self/{did}/place.stream.vod.draftVideo/{tid},
// so the tid is the final path segment. Used to reuse the draft's tid as the
// published video's rkey.
func draftTID(atsURI string) (string, error) {
	idx := strings.LastIndex(atsURI, "/")
	if idx < 0 || idx == len(atsURI)-1 {
		return "", fmt.Errorf("draft URI has no rkey: %s", atsURI)
	}
	return atsURI[idx+1:], nil
}

// publishTracksFromUpload publishes the place.stream.media.track records for a
// draft's tied upload, deferred from processing time to publish time. It reads
// the probe metadata + signing key + blob size + CID from the Upload row and
// calls publishTrack for each A/V stream, returning the fresh strongRefs to use
// as the video record's source. Returns (nil, nil) if the draft has no tied
// upload (so the caller can fall back to a carried-over source).
func publishTracksFromUpload(ctx context.Context, state *statedb.StatefulDB, client XRPCClient, did string, dv *statedb.DraftVideo) (*placestream.MediaDefs_SourceTracks, error) {
	if dv.OriginUploadID == "" {
		return nil, nil
	}
	upload, err := state.GetUpload(ctx, dv.OriginUploadID)
	if err != nil {
		return nil, fmt.Errorf("get upload: %w", err)
	}
	if upload == nil {
		return nil, nil
	}
	probe, err := unmarshalProbe(upload.ProbeJSON)
	if err != nil {
		return nil, err
	}
	if upload.ContentCID == "" {
		return nil, fmt.Errorf("upload %s has no content_cid", upload.ID)
	}
	var tracks []comatproto.RepoStrongRef
	if probe.Video != nil {
		ref, err := publishTrack(ctx, client, did, upload.ContentCID, upload.BlobSize, probe.DurationMS, "1", "video", upload.SigningKey, probe.Video, nil)
		if err != nil {
			return nil, fmt.Errorf("publish video track: %w", err)
		}
		tracks = append(tracks, *ref)
	}
	if probe.Audio != nil {
		ref, err := publishTrack(ctx, client, did, upload.ContentCID, upload.BlobSize, probe.DurationMS, "2", "audio", upload.SigningKey, nil, probe.Audio)
		if err != nil {
			return nil, fmt.Errorf("publish audio track: %w", err)
		}
		tracks = append(tracks, *ref)
	}
	if len(tracks) == 0 {
		return nil, nil
	}
	log.Log(ctx, "published media.track records (deferred to publish)",
		"uploadId", upload.ID, "cid", upload.ContentCID, "tracks", len(tracks))
	return &placestream.MediaDefs_SourceTracks{
		LexiconTypeID: "place.stream.media.defs#sourceTracks",
		Tracks:        tracks,
	}, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// draftConnectionsToVideo maps the draft's connections union slice to the video
// record's connections slice (both wrap place.stream.video#connection).
func draftConnectionsToVideo(in []placestream.VodDraftVideo_Connections_Elem) []placestream.Video_Connections_Elem {
	if len(in) == 0 {
		return nil
	}
	out := make([]placestream.Video_Connections_Elem, 0, len(in))
	for _, c := range in {
		if false || c.Video_Connection == nil {
			continue
		}
		out = append(out, placestream.Video_Connections_Elem{
			Video_Connection: c.Video_Connection,
		})
	}
	return out
}

// draftActivityToVideo maps the draft's activity union to the video record's.
func draftActivityToVideo(in *placestream.VodDraftVideo_Activity) *placestream.Video_Activity {
	if in == nil {
		return nil
	}
	if in.Defs_ActivityGame != nil {
		return &placestream.Video_Activity{Defs_ActivityGame: in.Defs_ActivityGame}
	}
	if in.Defs_ActivityLabel != nil {
		return &placestream.Video_Activity{Defs_ActivityLabel: in.Defs_ActivityLabel}
	}
	return nil
}
