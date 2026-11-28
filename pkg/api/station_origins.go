package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/constants"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// transferSourceURI verifies a public creator record and its strong reference
// to that same creator's indexed track. Clips do not independently authorize
// publication of the parent bytes. A clip must resolve through an eligible
// public parent (and cannot bypass its own labels).
func (a *StreamplaceAPI) transferSourceURI(ctx context.Context, rawURI, cid string) (string, error) {
	for depth := 0; depth <= 1; depth++ {
		uri, err := syntax.ParseATURI(rawURI)
		if err != nil || uri.Collection().String() != constants.PLACE_STREAM_VIDEO {
			return "", nil
		}
		allowed, err := vod.StationVideoAllowed(a.Model, rawURI)
		if err != nil || !allowed {
			return "", err
		}
		r, err := a.Model.GetVideoByURI(ctx, rawURI)
		if err != nil || r == nil {
			return "", err
		}
		if clip := r.Source.MediaDefs_SourceClip; clip != nil {
			rawURI = clip.Video
			continue
		}
		src := r.Source.MediaDefs_SourceTracks
		if src == nil || len(src.Tracks) == 0 {
			return "", nil
		}
		ref := src.Tracks[0]
		trackURI, err := syntax.ParseATURI(ref.Uri)
		if err != nil || trackURI.Authority() != uri.Authority() || trackURI.Collection().String() != constants.PLACE_STREAM_MEDIA_TRACK {
			return "", nil
		}
		track, err := a.Model.GetMediaTrackByURI(ctx, ref.Uri)
		if err != nil || track == nil || track.Track.MediaDefs_MuxlTrack == nil || track.Track.MediaDefs_MuxlTrack.Blob != cid {
			return "", err
		}
		actual, err := spid.GetCID(track)
		if err != nil || actual.String() != ref.Cid {
			return "", err
		}
		return uri.String(), nil
	}
	return "", nil
}

// Backfill starts from a public creator record, never a bucket scan, a draft,
// or the historical origin alone (old processors wrote those for drafts too).
// Local uploads additionally require the publisher's own persisted track refs;
// this prevents a user from guessing a private upload's CID in their repo.
func (a *StreamplaceAPI) backfillStationOrigins(ctx context.Context) (reindexOriginsResponse, error) {
	res := reindexOriginsResponse{ServerDID: a.CLI.StationDID}
	if a.PlaybackStore == nil || a.StatefulDB == nil {
		return res, fmt.Errorf("station playback store and state are required")
	}
	cursor := ""
	for {
		page, err := a.Model.GetVideoList(ctx, "", reindexOriginsPageSize, cursor, "")
		if err != nil {
			return res, err
		}
		for _, video := range page.Videos {
			res.Scanned++
			cid, _, err := a.resolveVideoContentBlob(ctx, video.Uri)
			if err != nil || cid == "" {
				continue
			}
			uri, err := a.transferSourceURI(ctx, video.Uri, cid)
			if err != nil {
				return res, err
			}
			if uri == "" || uri != video.Uri {
				continue
			}
			ok, err := a.legacyOriginEligible(ctx, uri, cid)
			if err != nil {
				return res, err
			}
			if !ok {
				continue
			}
			err = vod.AdvertiseStationVideo(ctx, a.PlaybackStore, res.ServerDID, cid, uri)
			switch {
			case err == nil:
				res.Indexed++
			case errors.Is(err, blob.ErrNotFound), errors.Is(err, vod.ErrStationOriginWithdrawn):
				// Missing media and withdrawals are not eligible for migration.
			default:
				res.Errors = append(res.Errors, uri+": "+err.Error())
			}
		}
		if page.Cursor == nil {
			return res, nil
		}
		cursor = *page.Cursor
	}
}

func (a *StreamplaceAPI) legacyOriginEligible(ctx context.Context, uri, cid string) (bool, error) {
	var uploads []statedb.Upload
	if err := a.StatefulDB.DB.WithContext(ctx).Select("user_did", "track_uris").Where("content_cid = ?", cid).Find(&uploads).Error; err != nil {
		return false, err
	}
	if len(uploads) == 0 {
		// Legacy operator transfers had no Upload row: require this node's
		// prior attestation, not a third-party origin or bare object.
		origin, err := a.Model.GetMediaOriginByURI(ctx, fmt.Sprintf("at://%s/%s/%s", a.CLI.ServerDID(), constants.PLACE_STREAM_MEDIA_ORIGIN, cid))
		return origin.Blob == cid, err
	}
	r, err := a.Model.GetVideoByURI(ctx, uri)
	if err != nil || r == nil || r.Source.MediaDefs_SourceTracks == nil || len(r.Source.MediaDefs_SourceTracks.Tracks) == 0 {
		return false, err
	}
	want := r.Source.MediaDefs_SourceTracks.Tracks[0]
	for _, upload := range uploads {
		if upload.RepoDID != syntax.ATURI(uri).Authority().String() || upload.TrackURIs == "" {
			continue
		}
		var refs []struct{ URI, CID string }
		if err := json.Unmarshal([]byte(upload.TrackURIs), &refs); err != nil {
			return false, err
		}
		for _, ref := range refs {
			if ref.URI == want.Uri && ref.CID == want.Cid {
				return true, nil
			}
		}
	}
	return false, nil
}

// HandleWithdrawOrigin runs only on the trusted internal admin router. The
// ACK is for the durable serving decision, not for eventual PDS deletion.
func (a *StreamplaceAPI) HandleWithdrawOrigin(_ context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if a.CLI.StationDID == "" || a.PlaybackStore == nil {
			apierrors.WriteHTTPBadRequest(w, "external station not configured", nil)
			return
		}
		var input struct {
			CID string `json:"cid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.CID == "" {
			apierrors.WriteHTTPBadRequest(w, "cid is required", err)
			return
		}
		if _, err := bdasl.Parse(input.CID); err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid origin CID", err)
			return
		}
		if err := vod.WithdrawStationOrigin(r.Context(), a.PlaybackStore, a.CLI.StationDID, input.CID); err != nil {
			apierrors.WriteHTTPInternalServerError(w, "persist station withdrawal", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
