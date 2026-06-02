package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"

	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/vod"
)

// vodTransferRequest is the body of POST /vod-transfer. Either resolve the
// content blob from a place.stream.video record (Source + URI) or name the
// blob directly (CID + DID); Source is always required.
type vodTransferRequest struct {
	// Source is the base URL of the node to pull the blob from
	// (e.g. "https://source.example").
	Source string `json:"source"`
	// URI is a place.stream.video AT-URI. When set, the content CID and
	// owning DID are resolved from the local index.
	URI string `json:"uri,omitempty"`
	// CID is the content blob's BDASL CID, an alternative to URI for when
	// the record isn't locally indexed. Requires DID.
	CID string `json:"cid,omitempty"`
	// DID is the account that owns a track in the blob; required alongside
	// CID (and ignored when URI is set, since it's derived from the record).
	DID string `json:"did,omitempty"`
}

// vodTransferHTTPClient is used for the (potentially long, multi-gigabyte)
// blob fetch. No client-level timeout — the request context bounds it — but
// a generous dial/handshake timeout so an unreachable source fails fast.
var vodTransferHTTPClient = &http.Client{
	Transport: &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

// HandleVODTransfer is the internal admin endpoint that pulls a VOD blob
// from another Streamplace node into this node's playback store and
// publishes a place.stream.media.origin for it. See vod.TransferVOD for the
// mechanics; this just resolves the request into (contentCID, did) and
// delegates.
func (a *StreamplaceAPI) HandleVODTransfer(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		reqCtx := r.Context()

		if a.PlaybackStore == nil {
			errors.WriteHTTPInternalServerError(w, "playback store not configured", nil)
			return
		}

		var req vodTransferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		if req.Source == "" {
			errors.WriteHTTPBadRequest(w, "source is required", nil)
			return
		}

		contentCID := req.CID
		did := req.DID
		switch {
		case req.URI != "":
			var err error
			contentCID, did, err = a.resolveVideoContentBlob(reqCtx, req.URI)
			if err != nil {
				errors.WriteHTTPBadRequest(w, fmt.Sprintf("resolve %s", req.URI), err)
				return
			}
		case req.CID != "" && req.DID != "":
			// Used as supplied.
		default:
			errors.WriteHTTPBadRequest(w, "provide either uri, or both cid and did", nil)
			return
		}

		log.Log(reqCtx, "vod transfer requested", "source", req.Source, "uri", req.URI, "cid", contentCID, "did", did)

		result, err := vod.TransferVOD(reqCtx, a.CLI, a.PlaybackStore, vodTransferHTTPClient, req.Source, contentCID, did)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "vod transfer failed", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Error(reqCtx, "error writing vod-transfer response", "error", err)
		}
	}
}

// resolveVideoContentBlob walks a place.stream.video record in the local
// index to the content blob a transfer should fetch, returning the blob's
// BDASL CID and the DID that owns a track in it (required by the source's
// getVideoBlob).
//
// Mirrors the playback resolver (pkg/spxrpc resolveVideoBlob): a
// sourceTracks record resolves to its first track's muxlTrack.blob; a
// sourceClip resolves through to its parent video (one level only), whose
// blob is the one actually backing the clip.
func (a *StreamplaceAPI) resolveVideoContentBlob(ctx context.Context, rawURI string) (cid string, did string, err error) {
	aturi, err := syntax.ParseATURI(rawURI)
	if err != nil {
		return "", "", fmt.Errorf("invalid AT-URI: %w", err)
	}

	rec, err := a.Model.GetVideoByURI(ctx, aturi.String())
	if err != nil {
		return "", "", fmt.Errorf("get video: %w", err)
	}
	if rec == nil {
		return "", "", fmt.Errorf("video not indexed locally (pass cid + did instead)")
	}

	switch {
	case rec.Source != nil && rec.Source.MediaDefs_SourceTracks != nil:
		cid, err = a.firstTrackBlobCID(ctx, rec.Source.MediaDefs_SourceTracks.Tracks)
		if err != nil {
			return "", "", err
		}
		return cid, aturi.Authority().String(), nil

	case rec.Source != nil && rec.Source.MediaDefs_SourceClip != nil:
		clip := rec.Source.MediaDefs_SourceClip
		if clip.Video == "" {
			return "", "", fmt.Errorf("sourceClip missing parent video URI")
		}
		parentURI, err := syntax.ParseATURI(clip.Video)
		if err != nil {
			return "", "", fmt.Errorf("invalid parent video URI: %w", err)
		}
		parent, err := a.Model.GetVideoByURI(ctx, parentURI.String())
		if err != nil {
			return "", "", fmt.Errorf("get parent video: %w", err)
		}
		if parent == nil {
			return "", "", fmt.Errorf("parent video %s not indexed locally", parentURI.String())
		}
		if parent.Source == nil || parent.Source.MediaDefs_SourceTracks == nil {
			return "", "", fmt.Errorf("sourceClip parent must be a sourceTracks video (clip-of-clip unsupported)")
		}
		cid, err = a.firstTrackBlobCID(ctx, parent.Source.MediaDefs_SourceTracks.Tracks)
		if err != nil {
			return "", "", err
		}
		// The blob lives in the parent's tracks, so attribute to the parent's owner.
		return cid, parentURI.Authority().String(), nil

	default:
		return "", "", fmt.Errorf("video record has no playable source")
	}
}

// firstTrackBlobCID resolves the muxlTrack.blob CID of the first track ref
// in a sourceTracks bundle via the local index. The metafile keyed at that
// CID catalogs every track of the container, so the first is enough.
func (a *StreamplaceAPI) firstTrackBlobCID(ctx context.Context, tracks []*comatproto.RepoStrongRef) (string, error) {
	if len(tracks) == 0 {
		return "", fmt.Errorf("video record has no tracks")
	}
	first := tracks[0]
	if first == nil || first.Uri == "" {
		return "", fmt.Errorf("first track ref is empty")
	}
	track, err := a.Model.GetMediaTrackByURI(ctx, first.Uri)
	if err != nil {
		return "", fmt.Errorf("get track %s: %w", first.Uri, err)
	}
	if track == nil || track.Track == nil || track.Track.MediaDefs_MuxlTrack == nil {
		return "", fmt.Errorf("track %s not indexed or not a muxlTrack", first.Uri)
	}
	blob := track.Track.MediaDefs_MuxlTrack.Blob
	if blob == "" {
		return "", fmt.Errorf("track %s has no blob CID", first.Uri)
	}
	return blob, nil
}
