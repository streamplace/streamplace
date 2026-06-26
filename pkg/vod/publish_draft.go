package vod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
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
	video := &streamplace.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         rec.Title,
		Description:   rec.Description,
		DurationMs:    derefInt64(rec.DurationMs),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if rec.Description != nil {
		video.Description = rec.Description
	}
	video.DescriptionFacets = rec.DescriptionFacets
	video.Tags = rec.Tags
	video.Connections = draftConnectionsToVideo(rec.Connections)
	video.Activity = draftActivityToVideo(rec.Activity)
	video.ContentWarnings = rec.ContentWarnings
	video.ContentRights = rec.ContentRights
	video.Thumb = rec.Thumb

	// Carry over the source union (the published track refs).
	if rec.Source != nil && rec.Source.MediaDefs_SourceTracks != nil {
		video.Source = &streamplace.Video_Source{
			MediaDefs_SourceTracks: rec.Source.MediaDefs_SourceTracks,
		}
	} else if rec.Source != nil && rec.Source.MediaDefs_SourceClip != nil {
		video.Source = &streamplace.Video_Source{
			MediaDefs_SourceClip: rec.Source.MediaDefs_SourceClip,
		}
	}

	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		span.RecordError(err)
		return "", "", fmt.Errorf("get user xrpc client: %w", err)
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

	rkey := spid.TIDClock.Next().String()
	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_VIDEO,
		Record:     &lexutil.LexiconTypeDecoder{Val: video},
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
func unmarshalDraftRecord(data []byte) (*streamplace.VodDraftVideo, error) {
	var rec streamplace.VodDraftVideo
	if err := rec.UnmarshalCBOR(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("unmarshal draft CBOR: %w", err)
	}
	return &rec, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// draftConnectionsToVideo maps the draft's connections union slice to the video
// record's connections slice (both wrap place.stream.video#connection).
func draftConnectionsToVideo(in []*streamplace.VodDraftVideo_Connections_Elem) []*streamplace.Video_Connections_Elem {
	if len(in) == 0 {
		return nil
	}
	out := make([]*streamplace.Video_Connections_Elem, 0, len(in))
	for _, c := range in {
		if c == nil || c.Video_Connection == nil {
			continue
		}
		out = append(out, &streamplace.Video_Connections_Elem{
			Video_Connection: c.Video_Connection,
		})
	}
	return out
}

// draftActivityToVideo maps the draft's activity union to the video record's.
func draftActivityToVideo(in *streamplace.VodDraftVideo_Activity) *streamplace.Video_Activity {
	if in == nil {
		return nil
	}
	if in.Defs_ActivityGame != nil {
		return &streamplace.Video_Activity{Defs_ActivityGame: in.Defs_ActivityGame}
	}
	if in.Defs_ActivityLabel != nil {
		return &streamplace.Video_Activity{Defs_ActivityLabel: in.Defs_ActivityLabel}
	}
	return nil
}
