package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// clipTTL is how long an ephemeral clip is available before the file and
// draft row are garbage-collected.
const clipTTL = 10 * time.Minute

// clipRateLimitPerViewer is the minimum interval between clip requests from
// the same viewer.
const clipRateLimitPerViewer = 30 * time.Second

// clipMaxConcurrentPerStream bounds how many clip muxes can run at once for
// the same streamer's buffer.
const clipMaxConcurrentPerStream = 3

// clipRateLimiter enforces per-viewer cooldowns and per-stream concurrency
// caps for clip creation.
type clipRateLimiter struct {
	mu              sync.Mutex
	lastClipPerUser map[string]time.Time
	activePerStream map[string]int
}

func newClipRateLimiter() *clipRateLimiter {
	return &clipRateLimiter{
		lastClipPerUser: make(map[string]time.Time),
		activePerStream: make(map[string]int),
	}
}

// allowViewer checks and updates the per-viewer cooldown. Returns false if
// the viewer is clipping too fast.
func (r *clipRateLimiter) allowViewer(did string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	last, ok := r.lastClipPerUser[did]
	if ok && now.Sub(last) < clipRateLimitPerViewer {
		return false
	}
	r.lastClipPerUser[did] = now
	return true
}

// acquireStream increments the per-stream active count. Returns false if
// the cap is reached. Caller must releaseStream when done.
func (r *clipRateLimiter) acquireStream(streamerDID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activePerStream[streamerDID] >= clipMaxConcurrentPerStream {
		return false
	}
	r.activePerStream[streamerDID]++
	return true
}

func (r *clipRateLimiter) releaseStream(streamerDID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activePerStream[streamerDID] > 0 {
		r.activePerStream[streamerDID]--
	}
}

// handlePlaceStreamClipCreate creates an ephemeral clip from a streamer's
// live broadcast.
func (s *Server) handlePlaceStreamClipCreate(ctx context.Context, input *placestream.ClipCreate_Input) (*placestream.ClipCreate_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}

	// Validate streamer DID.
	streamerDID := input.Streamer
	if err := validateDID(streamerDID); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}

	// Check that clipping is enabled for this streamer. Defaults to enabled
	// when unset (nil), matching livestreamRecording's behavior — a streamer
	// must explicitly set livestreamClipping: false to opt out.
	settings, err := s.model.GetServerSettings(ctx, s.cli.BroadcasterHost, streamerDID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get server settings: %v", err))
	}
	if settings != nil {
		spsettings, err := settings.ToStreamplaceServerSettings()
		if err == nil {
			if spsettings.LivestreamClipping != nil && !*spsettings.LivestreamClipping {
				return nil, echo.NewHTTPError(http.StatusForbidden, "clipping is disabled for this stream")
			}
		}
	}

	// Rate limit per viewer.
	if !s.clipLimiter.allowViewer(session.DID, time.Now()) {
		return nil, echo.NewHTTPError(http.StatusTooManyRequests, "you're clipping too fast, wait a moment")
	}

	// Rate limit per stream (concurrent mux cap).
	if !s.clipLimiter.acquireStream(streamerDID) {
		return nil, echo.NewHTTPError(http.StatusTooManyRequests, "too many clips being created for this stream, try again shortly")
	}
	defer s.clipLimiter.releaseStream(streamerDID)

	// Determine grab duration.
	durationMs := int64(60000)
	if input.DurationMs != nil {
		durationMs = *input.DurationMs
		if durationMs < 1000 {
			durationMs = 1000
		}
		if durationMs > 120000 {
			durationMs = 120000
		}
	}
	after := time.Now().Add(-time.Duration(durationMs) * time.Millisecond)

	// Generate clip ID.
	clipID, err := uuid.NewV7()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to generate clip ID")
	}

	// Mux from moderation buffer.
	filePath := []string{streamerDID, "clips", fmt.Sprintf("%s.mp4", clipID.String())}
	fd, err := s.cli.DataFileCreate(filePath, false)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to create clip file")
	}

	err = s.mm.ClipUser(ctx, streamerDID, fd, nil, &after)
	fd.Close()
	if err != nil {
		os.Remove(s.cli.DataFilePath(filePath))
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to clip: %v", err))
	}

	// Get file size to estimate actual duration.
	// The moderation buffer returns whatever segments are in the window,
	// so the actual content may be shorter than requested.
	fullPath := s.cli.DataFilePath(filePath)
	stat, err := os.Stat(fullPath)
	actualDurationMs := durationMs
	if err == nil && stat.Size() == 0 {
		os.Remove(fullPath)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "no content available to clip")
	}

	// Probe the muxed file so publish can create proper track records, and use
	// the probed duration when the buffer had fewer segments than requested. A
	// probe failure is non-fatal — the clip still previews and publishes, just
	// without track metadata.
	probeJSON := ""
	if f, perr := os.Open(fullPath); perr == nil {
		probe, perr := media.ProbeClipFile(f)
		f.Close()
		if perr == nil {
			if probe.DurationMS > 0 {
				actualDurationMs = probe.DurationMS
			}
			probeJSON, perr = vod.MarshalProbe(probe)
			if perr != nil {
				log.Warn(ctx, "failed to marshal clip probe", "error", perr)
			}
		} else {
			log.Warn(ctx, "failed to probe clip file", "error", perr)
		}
	}

	// Look up the streamer's active signing key so publish can create signed
	// track records. A missing key is non-fatal.
	signingKey := ""
	keys, kerr := s.model.GetSigningKeysForRepo(streamerDID)
	if kerr == nil {
		var best time.Time
		for i := range keys {
			if keys[i].RevokedAt != nil {
				continue
			}
			if keys[i].CreatedAt.After(best) {
				best = keys[i].CreatedAt
				signingKey = keys[i].DID
			}
		}
	}
	if signingKey == "" {
		log.Warn(ctx, "no active signing key for streamer", "streamer", streamerDID, "err", kerr)
	}

	// Create clip draft row.
	now := time.Now()
	expiresAt := now.Add(clipTTL)
	draft := &statedb.ClipDraft{
		ID:          clipID.String(),
		ClipperDID:  session.DID,
		StreamerDID: streamerDID,
		FilePath:    fullPath,
		DurationMs:  actualDurationMs,
		SigningKey:  signingKey,
		ProbeJSON:   probeJSON,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}
	if err := s.statefulDB.CreateClipDraft(ctx, draft); err != nil {
		os.Remove(s.cli.DataFilePath(filePath))
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create clip draft: %v", err))
	}

	previewURL := fmt.Sprintf("https://%s/api/clip/%s/%s.mp4", s.cli.BroadcasterHost, streamerDID, clipID.String())

	log.Log(ctx, "clip created", "clipId", clipID.String(), "streamer", streamerDID, "clipper", session.DID, "durationMs", actualDurationMs)

	return &placestream.ClipCreate_Output{
		ClipId:     clipID.String(),
		PreviewUrl: previewURL,
		ExpiresAt:  expiresAt.Format(time.RFC3339),
		DurationMs: actualDurationMs,
	}, nil
}

// handlePlaceStreamClipPublish publishes a clip as a place.stream.video +
// place.stream.clip.entry record pair.
func (s *Server) handlePlaceStreamClipPublish(ctx context.Context, input *placestream.ClipPublish_Input) (*placestream.ClipPublish_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}

	// Load clip draft.
	draft, err := s.statefulDB.GetClipDraft(ctx, input.ClipId)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get clip draft: %v", err))
	}
	if draft == nil || draft.ClipperDID != session.DID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "clip not found")
	}
	if draft.Published {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "clip already published")
	}

	// Check expiry.
	if time.Now().After(draft.ExpiresAt) {
		return nil, echo.NewHTTPError(http.StatusGone, "clip has expired")
	}

	// Verify the ephemeral file still exists.
	if _, err := os.Stat(draft.FilePath); err != nil {
		return nil, echo.NewHTTPError(http.StatusGone, "clip file no longer available")
	}

	// Process the ephemeral file and publish records.
	// This delegates to the vod package's publish infrastructure:
	// content-addressing, blob upload, origin/track/video/clip records.
	videoURI, videoCID, err := s.publishClipVideo(ctx, session.DID, draft, input)
	if err != nil {
		log.Error(ctx, "failed to publish clip video", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to publish clip: %v", err))
	}

	// Create the clip entry record referencing the video.
	clipURI, clipCID, clipErr := s.publishClipEntry(ctx, session.DID, videoURI, videoCID, input)
	if clipErr != nil {
		log.Error(ctx, "failed to publish clip entry record", "error", clipErr, "videoUri", videoURI)
		// Video succeeded but clip record failed. Return the video URI
		// so the client can retry just the clip entry creation.
		return &placestream.ClipPublish_Output{
			VideoUri: videoURI,
			VideoCid: videoCID,
		}, nil
	}

	// Mark draft as published (file cleanup happens via TTL sweeper or
	// could be immediate now that both records exist).
	if err := s.statefulDB.MarkClipDraftPublished(ctx, input.ClipId); err != nil {
		log.Error(ctx, "failed to mark clip draft as published", "error", err)
	}

	// Clean up the ephemeral file.
	os.Remove(draft.FilePath)

	log.Log(ctx, "clip published", "clipId", input.ClipId, "videoUri", videoURI, "clipUri", clipURI)

	return &placestream.ClipPublish_Output{
		VideoUri: videoURI,
		VideoCid: videoCID,
		ClipUri:  &clipURI,
		ClipCid:  &clipCID,
	}, nil
}

// publishClipVideo content-addresses the ephemeral clip file, stores it as
// a blob, publishes an origin record, and creates a place.stream.video
// record via the existing PublishVideo path.
func (s *Server) publishClipVideo(ctx context.Context, did string, draft *statedb.ClipDraft, input *placestream.ClipPublish_Input) (string, string, error) {
	// Read the ephemeral file.
	data, err := os.ReadFile(draft.FilePath)
	if err != nil {
		return "", "", fmt.Errorf("read clip file: %w", err)
	}

	// Content-address it.
	cid := bdasl.CID(data)
	blobSize := int64(len(data))

	// Store the blob in the playback store.
	blobKey := vod.BlobsPrefix + cid + ".mp4"
	w, err := s.playbackStore.NewWriter(ctx, blobKey, "video/mp4")
	if err != nil {
		return "", "", fmt.Errorf("create blob writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return "", "", fmt.Errorf("write blob: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", "", fmt.Errorf("close blob writer: %w", err)
	}

	// Generate the playback sidecars (per-track init blobs + metafile JSON).
	// getVideoPlaylist reads blobs/<cid>.json to catalog tracks and will 404
	// "BlobNotFound" without it — publishing an unplayable clip is worse than
	// failing the publish, so a sidecar error aborts.
	if _, _, err := vod.EnsureSidecars(ctx, s.playbackStore, cid, blobSize); err != nil {
		return "", "", fmt.Errorf("build clip sidecars: %w", err)
	}

	// Publish an origin record attesting this server has the blob.
	originRec := &placestream.MediaOrigin{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
		Blob:          cid,
		Size:          blobSize,
		MimeType:      "video/mp4",
	}
	if err := atproto.CommitServerRepoRecord(ctx, s.cli, constants.PLACE_STREAM_MEDIA_ORIGIN, cid, originRec); err != nil {
		return "", "", fmt.Errorf("publish origin: %w", err)
	}

	// Create an upload row so PublishVideo can read probe/signing info.
	uploadID := "clip-" + draft.ID
	upload := &statedb.Upload{
		ID:               uploadID,
		RepoDID:          did,
		MimeType:         "video/mp4",
		Size:             blobSize,
		Backend:          "file",
		ProcessingStatus: "done",
		ContentCID:       cid,
		DurationMS:       draft.DurationMs,
		BlobSize:         blobSize,
		SigningKey:       draft.SigningKey,
		ProbeJSON:        draft.ProbeJSON,
	}
	if err := s.statefulDB.CreateUpload(ctx, upload); err != nil {
		return "", "", fmt.Errorf("create upload row: %w", err)
	}

	// Publish the track records from the draft's probe + signing key and store
	// their refs so PublishVideo attaches them as the video record's source.
	client, err := s.clipUserXRPCClient(ctx, did)
	if err != nil {
		return "", "", fmt.Errorf("get user xrpc client: %w", err)
	}
	tracks, err := vod.PublishTracksForUpload(ctx, client, did, upload)
	if err != nil {
		return "", "", fmt.Errorf("publish clip tracks: %w", err)
	}
	if len(tracks) > 0 {
		refs := make([]vod.TrackRefJSON, 0, len(tracks))
		for _, t := range tracks {
			refs = append(refs, vod.TrackRefJSON{URI: t.Uri, CID: t.Cid})
		}
		refsJSON, err := json.Marshal(refs)
		if err != nil {
			return "", "", fmt.Errorf("marshal track refs: %w", err)
		}
		if err := s.statefulDB.SetUploadTrackURIs(ctx, uploadID, string(refsJSON)); err != nil {
			return "", "", fmt.Errorf("store track refs: %w", err)
		}
	}

	// Build the video record from the clip's authoritative metadata.
	video := &placestream.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         input.Title,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if input.Description != nil {
		video.Description = input.Description
	}

	// Publish the video record (publishes tracks from the upload row).
	return vod.PublishVideo(ctx, s.statefulDB, s.playbackStore, did, uploadID, video)
}

// handlePlaceStreamClipCancel deletes an unpublished draft's ephemeral muxed
// file immediately (the client's explicit cancel) rather than leaving it for
// the 10-minute TTL sweep, and removes the draft row.
func (s *Server) handlePlaceStreamClipCancel(ctx context.Context, input *placestream.ClipCancel_Input) (*placestream.ClipCancel_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session required")
	}
	draft, err := s.statefulDB.GetClipDraft(ctx, input.ClipId)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get clip draft: %v", err))
	}
	if draft == nil || draft.ClipperDID != session.DID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "clip not found")
	}
	if draft.Published {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "clip already published")
	}
	os.Remove(draft.FilePath)
	if err := s.statefulDB.DeleteClipDraft(ctx, input.ClipId); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to delete clip draft: %v", err))
	}
	log.Log(ctx, "clip cancelled", "clipId", input.ClipId, "clipper", session.DID)
	return &placestream.ClipCancel_Output{Cancelled: true}, nil
}

// clipUserXRPCClient resolves a clip participant's (the clipper's) OAuth
// session and returns an authenticated xrpc client for record creation.
func (s *Server) clipUserXRPCClient(ctx context.Context, did string) (*oatproxy.XrpcClient, error) {
	client, err := s.statefulDB.GetSessionByDID(did)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("no session for %s", did)
	}
	client, err = s.op.RefreshIfNeeded(client)
	if err != nil {
		return nil, fmt.Errorf("refresh session: %w", err)
	}
	xrpcClient, err := s.op.GetXrpcClient(client)
	if err != nil {
		return nil, fmt.Errorf("get xrpc client: %w", err)
	}
	return xrpcClient, nil
}

// publishClipEntry creates a place.stream.clip.entry record in the clipper's
// repo referencing the published video and the source livestream.
func (s *Server) publishClipEntry(ctx context.Context, did, videoURI, videoCID string, input *placestream.ClipPublish_Input) (string, string, error) {
	xrpcClient, err := s.clipUserXRPCClient(ctx, did)
	if err != nil {
		return "", "", err
	}

	clip := &placestream.ClipEntry{
		LexiconTypeID: constants.PLACE_STREAM_CLIP_ENTRY,
		Video: comatproto.RepoStrongRef{
			Uri: videoURI,
			Cid: videoCID,
		},
		Livestream: input.Livestream,
		Start:      input.Start,
		End:        input.End,
		Title:      input.Title,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if input.Description != nil {
		clip.Description = input.Description
	}

	createInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CLIP_ENTRY,
		Record:     &glex.LexiconTypeDecoder{Val: clip},
		Repo:       did,
	}
	createOutput := comatproto.RepoCreateRecord_Output{}

	err = xrpcClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, createInput, &createOutput)
	if err != nil {
		return "", "", fmt.Errorf("create clip record: %w", err)
	}

	return createOutput.Uri, createOutput.Cid, nil
}
