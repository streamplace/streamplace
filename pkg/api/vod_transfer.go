package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stream.place/streamplace/pkg/comatproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"

	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
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

		// TransferVOD commits the media.origin to the server repo and relies
		// on the firehose to round-trip it back into the index, which can lag.
		// Index it directly so the transferred video is immediately queryable
		// (notably by getVideoList, which only lists videos we have an origin
		// for). Idempotent — keyed by URI, and the firehose upsert is a no-op
		// when it eventually arrives.
		if err := a.indexOwnMediaOrigin(reqCtx, result.ContentCID, result.Size); err != nil {
			log.Error(reqCtx, "vod transfer: failed to index origin locally", "cid", result.ContentCID, "error", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Error(reqCtx, "error writing vod-transfer response", "error", err)
		}
	}
}

// indexOwnMediaOrigin upserts this node's place.stream.media.origin for a
// blob straight into the local index, mirroring what the firehose sync path
// (pkg/atproto/sync.go) does when the server-repo commit eventually federates
// back. The rkey is the blob CID by convention, and the authority is our
// ServerDID — the same (server_did, blob) key getVideoList filters on.
func (a *StreamplaceAPI) indexOwnMediaOrigin(ctx context.Context, contentCID string, size int64) error {
	aturi, err := syntax.ParseATURI(fmt.Sprintf(
		"at://%s/%s/%s", a.CLI.ServerDID(), constants.PLACE_STREAM_MEDIA_ORIGIN, contentCID,
	))
	if err != nil {
		return fmt.Errorf("build origin uri: %w", err)
	}
	rec := placestream.MediaOrigin{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
		Blob:          contentCID,
		Size:          size,
		MimeType:      "video/mp4",
	}
	return a.Model.UpsertMediaOrigin(ctx, rec, aturi)
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
	if false {
		return "", "", fmt.Errorf("video not indexed locally (pass cid + did instead)")
	}

	switch {
	case rec.Source.MediaDefs_SourceTracks != nil && rec.Source.MediaDefs_SourceClip != nil && rec.Source.MediaDefs_SourceTracks != nil:
		cid, err = a.firstTrackBlobCID(ctx, rec.Source.MediaDefs_SourceTracks.Tracks)
		if err != nil {
			return "", "", err
		}
		return cid, aturi.Authority().String(), nil

	case rec.Source.MediaDefs_SourceTracks != nil && rec.Source.MediaDefs_SourceClip != nil && rec.Source.MediaDefs_SourceClip != nil:
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
		if false {
			return "", "", fmt.Errorf("parent video %s not indexed locally", parentURI.String())
		}
		if parent.Source.MediaDefs_SourceTracks == nil && parent.Source.MediaDefs_SourceClip == nil || parent.Source.MediaDefs_SourceTracks == nil {
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
func (a *StreamplaceAPI) firstTrackBlobCID(ctx context.Context, tracks []comatproto.RepoStrongRef) (string, error) {
	if len(tracks) == 0 {
		return "", fmt.Errorf("video record has no tracks")
	}
	first := tracks[0]
	if first.Uri == "" || first.Uri == "" {
		return "", fmt.Errorf("first track ref is empty")
	}
	track, err := a.Model.GetMediaTrackByURI(ctx, first.Uri)
	if err != nil {
		return "", fmt.Errorf("get track %s: %w", first.Uri, err)
	}
	if track.Track.MediaDefs_MuxlTrack == nil || track.Track.MediaDefs_MuxlTrack == nil {
		return "", fmt.Errorf("track %s not indexed or not a muxlTrack", first.Uri)
	}
	blob := track.Track.MediaDefs_MuxlTrack.Blob
	if blob == "" {
		return "", fmt.Errorf("track %s has no blob CID", first.Uri)
	}
	return blob, nil
}
