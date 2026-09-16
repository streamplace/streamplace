package vod

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// The operator's hands on a streamer's video records, for the streamer who
// can't (or shouldn't have to) do it from the app: read, retitle, repair and
// remove place.stream.video records, and the track records under them. Every
// write goes through the streamer's stored session, as PublishVideo's does,
// so nothing here can touch a repo whose owner never signed in to this node.

// ErrNotAVideo is returned for a URI that is not a place.stream.video record.
var ErrNotAVideo = errors.New("not a place.stream.video URI")

// VideoRecord is one video record as read from the streamer's PDS.
type VideoRecord struct {
	URI   string
	CID   string
	Video *placestream.Video
}

// TrackURIs are the record URIs a video's sourceTracks source points at; nil
// for a clip or a record without a source.
func (r VideoRecord) TrackURIs() []string {
	if r.Video == nil || r.Video.Source.MediaDefs_SourceTracks == nil {
		return nil
	}
	out := make([]string, 0, len(r.Video.Source.MediaDefs_SourceTracks.Tracks))
	for _, t := range r.Video.Source.MediaDefs_SourceTracks.Tracks {
		out = append(out, t.Uri)
	}
	return out
}

// GetVideoRecord reads one video record from the streamer's PDS.
func GetVideoRecord(ctx context.Context, state *statedb.StatefulDB, uri string) (*VideoRecord, error) {
	aturi, err := parseVideoURI(uri)
	if err != nil {
		return nil, err
	}
	did := aturi.Authority().String()
	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		return nil, err
	}
	return getVideoRecord(ctx, client, aturi)
}

func getVideoRecord(ctx context.Context, client XRPCClient, aturi syntax.ATURI) (*VideoRecord, error) {
	out := comatproto.RepoGetRecord_Output{}
	err := client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       aturi.Authority().String(),
		"collection": constants.PLACE_STREAM_VIDEO,
		"rkey":       aturi.RecordKey().String(),
	}, nil, &out)
	if err != nil {
		return nil, fmt.Errorf("getRecord %s: %w", aturi, err)
	}
	if out.Value == nil {
		return nil, fmt.Errorf("getRecord %s: empty value", aturi)
	}
	video, err := glex.RecordAs[placestream.Video](out.Value.Val)
	if err != nil {
		return nil, fmt.Errorf("getRecord %s: %w", aturi, err)
	}
	rec := &VideoRecord{URI: aturi.String(), Video: video}
	if out.Cid != nil {
		rec.CID = *out.Cid
	}
	return rec, nil
}

// ListVideoRecords reads every video record in a streamer's repo, newest
// rkey first (the PDS's order).
func ListVideoRecords(ctx context.Context, state *statedb.StatefulDB, did string) ([]VideoRecord, error) {
	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		return nil, err
	}
	var recs []VideoRecord
	var cursor *string
	for {
		params := map[string]any{"repo": did, "collection": constants.PLACE_STREAM_VIDEO, "limit": 100}
		if cursor != nil {
			params["cursor"] = *cursor
		}
		out := comatproto.RepoListRecords_Output{}
		if err := client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.listRecords", params, nil, &out); err != nil {
			return nil, fmt.Errorf("listRecords %s: %w", did, err)
		}
		for _, r := range out.Records {
			rec := VideoRecord{URI: r.Uri, CID: r.Cid}
			if r.Value != nil {
				if v, err := glex.RecordAs[placestream.Video](r.Value.Val); err == nil {
					rec.Video = v
				}
			}
			recs = append(recs, rec)
		}
		if out.Cursor == nil || *out.Cursor == "" || len(out.Records) == 0 {
			break
		}
		cursor = out.Cursor
	}
	return recs, nil
}

// VideoUpdate is what UpdateVideo changes on a record. A nil pointer leaves
// the field alone; an empty description or tag list clears it. UploadID
// names a finished upload whose tracks become the record's source (and whose
// duration its durationMs): the repair for a record published without them,
// or the way to point a record at a re-finalized VOD.
type VideoUpdate struct {
	Title       *string
	Description *string
	Tags        []string
	UploadID    string
}

// applyVideoUpdate applies the text fields of an update; it reports whether
// anything changed.
func applyVideoUpdate(v *placestream.Video, up VideoUpdate) bool {
	changed := false
	if up.Title != nil {
		if t := strings.TrimSpace(*up.Title); t != "" && t != v.Title {
			v.Title = t
			changed = true
		}
	}
	if up.Description != nil {
		d := strings.TrimSpace(*up.Description)
		switch {
		case d == "" && v.Description != nil:
			v.Description = nil
			v.DescriptionFacets = nil
			changed = true
		case d != "" && (v.Description == nil || *v.Description != d):
			v.Description = &d
			v.DescriptionFacets = nil
			changed = true
		}
	}
	if up.Tags != nil {
		tags := make([]string, 0, len(up.Tags))
		for _, t := range up.Tags {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
		if len(tags) == 0 {
			tags = nil
		}
		if strings.Join(tags, "\x00") != strings.Join(v.Tags, "\x00") {
			v.Tags = tags
			changed = true
		}
	}
	return changed
}

// UpdateVideo rewrites a video record in place (same rkey, compare-and-swap
// on the CID it read) with the given changes, and returns the new CID. With
// an UploadID, the record's source becomes that upload's track records
// (published now if the upload has none yet), its durationMs the upload's,
// and a missing thumbnail is generated from the upload's content, as
// PublishVideo would have done.
func UpdateVideo(ctx context.Context, state *statedb.StatefulDB, store blob.Store, uri string, up VideoUpdate) (string, error) {
	aturi, err := parseVideoURI(uri)
	if err != nil {
		return "", err
	}
	did := aturi.Authority().String()
	ctx = log.WithLogValues(ctx, "func", "UpdateVideo", "did", did, "uri", uri)
	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		return "", err
	}
	rec, err := getVideoRecord(ctx, client, aturi)
	if err != nil {
		return "", err
	}
	video := rec.Video
	changed := applyVideoUpdate(video, up)

	if up.UploadID != "" {
		upload, err := state.GetUpload(ctx, up.UploadID)
		if err != nil {
			return "", fmt.Errorf("get upload: %w", err)
		}
		if upload == nil || upload.RepoDID != did {
			return "", ErrUploadNotFound
		}
		if upload.ProcessingStatus != "done" {
			return "", ErrUploadNotReady
		}
		tracks, err := tracksForUpload(ctx, state, client, did, upload)
		if err != nil {
			return "", fmt.Errorf("publish tracks: %w", err)
		}
		if tracks == nil {
			return "", fmt.Errorf("upload %s has no playable streams to publish tracks for", upload.ID)
		}
		video.Source = placestream.Video_Source{MediaDefs_SourceTracks: tracks}
		video.DurationMs = upload.DurationMS
		if video.Thumb == nil && store != nil && upload.ContentCID != "" {
			thumb, terr := generateAndUploadThumbnail(ctx, client, store, upload.ContentCID)
			if terr != nil {
				log.Warn(ctx, "UpdateVideo: thumbnail backfill failed; updating without thumb", "error", terr)
			} else {
				video.Thumb = thumb
			}
		}
		changed = true
	}
	if !changed {
		return rec.CID, nil
	}

	var swap *string
	if rec.CID != "" {
		swap = &rec.CID
	}
	inp := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_VIDEO,
		Record:     &glex.LexiconTypeDecoder{Val: video},
		Rkey:       aturi.RecordKey().String(),
		Repo:       did,
		SwapRecord: swap,
	}
	out := comatproto.RepoPutRecord_Output{}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		return "", fmt.Errorf("putRecord video: %w", err)
	}
	log.Log(ctx, "updated video record (operator)", "uri", uri, "cid", out.Cid, "title", video.Title, "upload", up.UploadID)
	return out.Cid, nil
}

// DeleteVideo deletes a video record and, when withTracks, the track records
// its source pointed at (in the same repo). It returns the URIs it deleted.
// The content blob and the upload row stay: another record can be published
// for the same upload.
func DeleteVideo(ctx context.Context, state *statedb.StatefulDB, uri string, withTracks bool) ([]string, error) {
	aturi, err := parseVideoURI(uri)
	if err != nil {
		return nil, err
	}
	did := aturi.Authority().String()
	ctx = log.WithLogValues(ctx, "func", "DeleteVideo", "did", did, "uri", uri)
	client, err := getUserXRPCClient(ctx, state, did)
	if err != nil {
		return nil, err
	}
	rec, err := getVideoRecord(ctx, client, aturi)
	if err != nil {
		return nil, err
	}
	var deleted []string
	if err := deleteRecord(ctx, client, aturi); err != nil {
		return nil, err
	}
	deleted = append(deleted, aturi.String())
	if withTracks {
		for _, t := range rec.TrackURIs() {
			turi, err := syntax.ParseATURI(t)
			if err != nil || turi.Authority().String() != did {
				log.Warn(ctx, "not deleting a track outside the video's repo", "track", t)
				continue
			}
			if err := deleteRecord(ctx, client, turi); err != nil {
				return deleted, err
			}
			deleted = append(deleted, turi.String())
		}
	}
	log.Log(ctx, "deleted video record (operator)", "deleted", deleted)
	return deleted, nil
}

func deleteRecord(ctx context.Context, client XRPCClient, aturi syntax.ATURI) error {
	inp := comatproto.RepoDeleteRecord_Input{
		Collection: aturi.Collection().String(),
		Repo:       aturi.Authority().String(),
		Rkey:       aturi.RecordKey().String(),
	}
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", map[string]any{}, inp, nil); err != nil {
		return fmt.Errorf("deleteRecord %s: %w", aturi, err)
	}
	return nil
}

// parseVideoURI accepts an at:// URI of a place.stream.video record.
func parseVideoURI(uri string) (syntax.ATURI, error) {
	aturi, err := syntax.ParseATURI(strings.TrimSpace(uri))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotAVideo, err)
	}
	if aturi.Collection().String() != constants.PLACE_STREAM_VIDEO || aturi.RecordKey().String() == "" {
		return "", fmt.Errorf("%w: %s", ErrNotAVideo, uri)
	}
	return aturi, nil
}
