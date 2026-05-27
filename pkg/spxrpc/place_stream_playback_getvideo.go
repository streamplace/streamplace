package spxrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/pkg/vod"
)

// sessionIDOrNew returns the caller's sid if it's well-formed, otherwise
// gives you a fresh tid
func sessionIDOrNew(supplied string) (string, error) {
	if supplied != "" {
		_, err := syntax.ParseTID(supplied)
		if err != nil {
			return "", fmt.Errorf("invalid session ID: %w", err)
		}
		return supplied, nil
	}
	return spid.TID(), nil
}

// videoCollection is the only collection getVideoPlaylist currently
// knows how to dispatch on. The lexicon accepts any AT-URI so we have
// room to grow into other playable record types without a wire break.
const videoCollection = "place.stream.video"

// --- stubs for the auto-generated wrappers in stubs.go ------------------
//
// The lexgen-generated stubs always status-200 + hard-code Content-Type
// via c.Stream, neither of which works for these endpoints
// (Range / 206 Partial Content on getVideoBlob, vnd.apple.mpegurl on
// getVideoPlaylist). We register custom echo routes in NewServer that
// override the stubs; these methods exist only to satisfy the build.

// stubMisrouted returns the "auto-stub fired" error. Factored out into
// a function whose return value isn't statically a known non-nil so
// the auto-generated wrapper's `if err != nil` check doesn't trip
// staticcheck's SA4023 (always-true comparison).
func stubMisrouted(name string) error {
	if name == "" {
		return nil
	}
	return echo.NewHTTPError(http.StatusInternalServerError,
		name+" auto-stub should have been overridden by NewServer; this is a wiring bug")
}

func (s *Server) handlePlaceStreamPlaybackGetVideoBlob(ctx context.Context, cid string, did string, sid string) (io.Reader, error) {
	return nil, stubMisrouted("getVideoBlob")
}

func (s *Server) handlePlaceStreamPlaybackGetVideoPlaylist(ctx context.Context, end *int, sid string, start *int, track string, uri string) (io.Reader, error) {
	return nil, stubMisrouted("getVideoPlaylist")
}

// --- getVideoBlob -------------------------------------------------------

// HandleGetVideoBlob serves a content-addressed playback blob (primary
// fMP4 or per-track init segment, indistinguishable from the
// endpoint's POV) with HTTP Range support.
//
// Strips the cosmetic `.m4s` / `.mp4` suffix on the way in: ffmpeg's
// HLS demuxer checks the URL extension against an allowlist
// (`allowed_segment_extensions`) before fetching segments. The
// playlist generator appends `.m4s` to every URL it emits so
// ffmpeg-based players don't refuse them; the handler ignores the
// suffix entirely.
//
// `did` is the owning account, carried by the playlist. For a known
// content blob it's verified against the MediaTrack index — a DID that
// doesn't own a track in the blob is rejected — and a ban on that
// account makes the blob unavailable. The blob is still served purely
// by CID; the (did, cid) pairing is just what makes labeler enforcement
// spoof-resistant. Per-track init segments aren't in any MediaTrack and
// serve unguarded (they're useless without the gated content blob).
func (s *Server) HandleGetVideoBlob(c echo.Context) error {
	ctx := c.Request().Context()
	cid := strings.TrimSuffix(c.QueryParam("cid"), ".m4s")
	cid = strings.TrimSuffix(cid, ".mp4")
	if cid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "cid is required")
	}
	parsedDID, err := syntax.ParseDID(c.QueryParam("did"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "did is required and must be a valid DID")
	}
	did := parsedDID.String()

	// Labeler enforcement. If this CID is a known content blob, the
	// supplied `did` must actually own a track in it (a spoofed DID has
	// no matching MediaTrack and fails as not-found), and that account
	// must not be banned. CIDs with no MediaTrack — per-track init
	// segments, or simply unknown CIDs — serve unguarded.
	tracks, err := s.model.GetMediaTracksByBlob(ctx, cid)
	if err != nil {
		log.Error(ctx, "playback: GetMediaTracksByBlob failed", "cid", cid, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if len(tracks) > 0 {
		owned := false
		for _, t := range tracks {
			if t.RepoDID == did {
				owned = true
				break
			}
		}
		if !owned {
			return echo.NewHTTPError(http.StatusNotFound, "BlobNotFound")
		}
		if banned, err := s.accountBanned(did); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		} else if banned {
			return echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
		}
	}

	if s.playbackStore == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	key := vod.BlobsPrefix + cid + ".mp4"
	r, err := s.playbackStore.Open(ctx, key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "BlobNotFound")
		}
		log.Error(ctx, "playback: open blob failed", "cid", cid, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer r.Close()
	rangeStart, rangeEnd := parseRangeForLog(c.Request().Header.Get("Range"), r.Size())
	s.logSegmentRequest(c, cid, did, c.QueryParam("sid"), rangeStart, rangeEnd)
	return serveBlobRange(c, r, "video/mp4")
}

// parseRangeForLog is the lenient counterpart of parseSingleRange used
// only for the view-log. When the request carries no Range header it
// served the whole blob — return [0, size-1] so the aggregator can
// credit every track's bytes. When the header is malformed (the
// serving path 416's) we still log [0, 0] which the aggregator treats
// as zero-bytes; logging shouldn't propagate parse failures.
// End is inclusive in both cases, matching RFC 7233.
func parseRangeForLog(header string, size int64) (int64, int64) {
	if header == "" {
		if size <= 0 {
			return 0, 0
		}
		return 0, size - 1
	}
	start, end, err := parseSingleRange(header, size)
	if err != nil {
		return 0, 0
	}
	return start, end
}

// serveBlobRange writes a blob.Reader to the echo response, honoring
// HTTP Range. We only support single ranges (`bytes=N-M`, `bytes=N-`,
// `bytes=-N`); multi-range requests are rejected with 416. Cache
// headers are set immutable since the URL is content-addressed.
func serveBlobRange(c echo.Context, r blob.Reader, contentType string) error {
	size := r.Size()
	rangeHeader := c.Request().Header.Get("Range")

	h := c.Response().Header()
	h.Set("Content-Type", contentType)
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "public, max-age=31536000, immutable")

	if rangeHeader == "" {
		h.Set("Content-Length", strconv.FormatInt(size, 10))
		c.Response().WriteHeader(http.StatusOK)
		_, err := io.Copy(c.Response().Writer, io.NewSectionReader(r, 0, size))
		return err
	}

	start, end, err := parseSingleRange(rangeHeader, size)
	if err != nil {
		h.Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return echo.NewHTTPError(http.StatusRequestedRangeNotSatisfiable, err.Error())
	}
	length := end - start + 1
	h.Set("Content-Length", strconv.FormatInt(length, 10))
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	c.Response().WriteHeader(http.StatusPartialContent)
	_, err = io.Copy(c.Response().Writer, io.NewSectionReader(r, start, length))
	return err
}

// parseSingleRange handles the three valid single-range forms for a
// known content size and returns absolute [start, end] inclusive
// bounds. Multi-range ("bytes=0-100,200-300") is rejected.
func parseSingleRange(header string, size int64) (int64, int64, error) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, fmt.Errorf("unsupported range unit (only bytes)")
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, fmt.Errorf("multi-range not supported")
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, fmt.Errorf("missing '-' in range spec")
	}
	startStr, endStr := spec[:dash], spec[dash+1:]
	switch {
	case startStr == "" && endStr != "":
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, fmt.Errorf("bad suffix range")
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, nil
	case startStr != "" && endStr == "":
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 || start >= size {
			return 0, 0, fmt.Errorf("start out of bounds")
		}
		return start, size - 1, nil
	default:
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 || start >= size {
			return 0, 0, fmt.Errorf("start out of bounds")
		}
		end, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, fmt.Errorf("end before start")
		}
		if end >= size {
			end = size - 1
		}
		return start, end, nil
	}
}

// --- getVideoPlaylist ---------------------------------------------------

// HandleGetVideoPlaylist resolves the place.stream.video record via
// the local index, dereferences its tracks + the metafile sidecar,
// and emits an HLS master or media playlist.
//
// Both playlist variants reference per-segment + init bytes via
// getVideoBlob?cid=... so playback is purely a function of the blob
// store plus the metafile — no extra round trips back to atproto
// after this single resolve.
func (s *Server) HandleGetVideoPlaylist(c echo.Context) error {
	ctx := c.Request().Context()
	rawURI := c.QueryParam("uri")
	if rawURI == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	aturi, err := s.normalizeURI(ctx, rawURI)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "uri must be a valid AT-URI: "+err.Error())
	}
	if aturi.Collection() != videoCollection {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("UnsupportedCollection: %s", aturi.Collection()))
	}
	if s.playbackStore == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}

	track := c.QueryParam("track")
	startMS, err := optionalInt64Param(c, "start")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	endMS, err := optionalInt64Param(c, "end")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if startMS != nil && endMS != nil && *startMS >= *endMS {
		return echo.NewHTTPError(http.StatusBadRequest, "start must be less than end")
	}

	// Reuse the caller's sid when present (i.e. the player followed a
	// URL we previously handed it) so the master/media/segment requests
	// of one playback session share an identifier. Generate a fresh one
	// when the player is making its first request.
	sid, err := sessionIDOrNew(c.QueryParam("sid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	uri := aturi.String()

	// Labeler enforcement: any label on the video record, or a ban on
	// the owning account, makes the VOD unavailable. This is the
	// playback entry point, so gating here blocks the whole flow before
	// any blob URLs are handed out.
	if labeled, err := s.recordLabeled(uri); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if labeled {
		return echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}
	if banned, err := s.accountBanned(aturi.Authority().String()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}

	resolved, err := s.resolveVideoBlob(ctx, uri)
	if err != nil {
		return err
	}
	meta, err := s.fetchMetafile(ctx, resolved.blobCID)
	if err != nil {
		return err
	}

	// Compose query-param start/end with any record-level clip bounds.
	// Query params live in the *clip's* local timeline (0 == start of
	// the clip); filterSegments wants bounds in the parent video's
	// timeline. For non-clip records clipStartMS == 0 and clipEndMS ==
	// nil, so the math collapses to identity.
	effectiveStartMS, effectiveEndMS := composeClipBounds(resolved.clipStartMS, resolved.clipEndMS, startMS, endMS)

	var body string
	kind := "master"
	if track == "" {
		// Master playlist's sub-playlist URLs carry the *unmodified*
		// query-param start/end (clip-local). Each per-track follow-up
		// request re-resolves the clip and composes bounds again on the
		// server side, so the propagation stays in clip-local time.
		body = masterPlaylist(meta, uri, sid, startMS, endMS)
	} else {
		body, err = mediaPlaylist(meta, track, aturi.Authority().String(), sid, s.cli.VODCDNURL, effectiveStartMS, effectiveEndMS)
		if err != nil {
			return err
		}
		kind = "media"
	}
	s.logManifestRequest(c, uri, sid, track, kind)

	c.Response().Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Writer.Write([]byte(body))
	return err
}

// optionalInt64Param parses a query param as int64. Returns (nil, nil)
// when the param is absent, (nil, err) when it's present but malformed.
func optionalInt64Param(c echo.Context, name string) (*int64, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer", name)
	}
	return &n, nil
}

// resolvedVideo carries everything HandleGetVideoPlaylist needs to
// emit a playlist for a given video URI: the primary blob CID to fetch
// the metafile from, and (for sourceClip records) the parent-timeline
// bounds the playback should be restricted to.
//
// For direct sourceTracks records both clip fields are zero values
// (clipEndMS == nil meaning "no upper bound"); for sourceClip records
// they carry the bounds the record names, in milliseconds against the
// parent video's timeline.
type resolvedVideo struct {
	blobCID     string
	clipStartMS int64
	clipEndMS   *int64
}

// resolveVideoBlob loads the place.stream.video record at uri and walks
// it to a content blob the player can read segments out of.
//
// Two source shapes are supported, mirroring the union in the video
// lexicon:
//
//   - place.stream.media.defs#sourceTracks: the canonical case. We
//     grab the first track of the bundle and use its muxlTrack.blob
//     as the primary blob CID; the metafile keyed at that CID
//     catalogs every track in the same container.
//
//   - place.stream.media.defs#sourceClip: the record is a "clip" of
//     another video. We re-resolve the parent (which itself must be a
//     plain sourceTracks record — we don't chain clips) and surface
//     the clip's start/end millisecond bounds so the handler can
//     restrict the emitted segments.
func (s *Server) resolveVideoBlob(ctx context.Context, uri string) (*resolvedVideo, error) {
	videoRec, err := s.loadVideoRecord(ctx, uri)
	if err != nil {
		return nil, err
	}

	switch {
	case videoRec.Source != nil && videoRec.Source.MediaDefs_SourceTracks != nil:
		blobCID, err := s.firstTrackBlob(ctx, videoRec.Source.MediaDefs_SourceTracks)
		if err != nil {
			return nil, err
		}
		return &resolvedVideo{blobCID: blobCID}, nil

	case videoRec.Source != nil && videoRec.Source.MediaDefs_SourceClip != nil:
		clip := videoRec.Source.MediaDefs_SourceClip
		if clip.Video == "" {
			return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "sourceClip missing parent video URI")
		}
		if clip.Start < 0 || clip.End <= clip.Start {
			return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "sourceClip bounds must be 0 <= start < end")
		}
		parentRec, err := s.loadVideoRecord(ctx, clip.Video)
		if err != nil {
			return nil, err
		}
		// One level only — clip-of-clip is rejected to keep playback
		// resolution bounded and predictable.
		if parentRec.Source == nil || parentRec.Source.MediaDefs_SourceTracks == nil {
			return nil, echo.NewHTTPError(http.StatusUnprocessableEntity,
				"sourceClip's parent video must use sourceTracks (clip-of-clip is not supported)")
		}
		blobCID, err := s.firstTrackBlob(ctx, parentRec.Source.MediaDefs_SourceTracks)
		if err != nil {
			return nil, err
		}
		end := clip.End
		return &resolvedVideo{
			blobCID:     blobCID,
			clipStartMS: clip.Start,
			clipEndMS:   &end,
		}, nil

	default:
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "video record has no playable source")
	}
}

// loadVideoRecord pulls the place.stream.video record at uri out of
// the local index. Returns a typed error suitable for surfacing back
// to the client (404, 500).
func (s *Server) loadVideoRecord(ctx context.Context, uri string) (*streamplace.Video, error) {
	videoRec, err := s.model.GetVideoByURI(ctx, uri)
	if err != nil {
		log.Error(ctx, "playback: GetVideoByURI failed", "uri", uri, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if videoRec == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "VideoNotFound")
	}
	return videoRec, nil
}

// firstTrackBlob returns the muxlTrack.blob CID of the first ref in a
// sourceTracks bundle. The metafile at that CID describes every track
// in the same container, so any track's blob is enough.
func (s *Server) firstTrackBlob(ctx context.Context, src *streamplace.MediaDefs_SourceTracks) (string, error) {
	if len(src.Tracks) == 0 {
		return "", echo.NewHTTPError(http.StatusUnprocessableEntity, "video record has no tracks")
	}
	firstRef := src.Tracks[0]
	if firstRef == nil || firstRef.Uri == "" {
		return "", echo.NewHTTPError(http.StatusUnprocessableEntity, "video record's first track ref is empty")
	}
	track, err := s.model.GetMediaTrackByURI(ctx, firstRef.Uri)
	if err != nil {
		log.Error(ctx, "playback: GetMediaTrackByURI failed", "uri", firstRef.Uri, "error", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil || track.Track == nil || track.Track.MediaDefs_MuxlTrack == nil {
		return "", echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}
	blob := track.Track.MediaDefs_MuxlTrack.Blob
	if blob == "" {
		return "", echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}
	return blob, nil
}

// fetchMetafile reads blobs/<cid>.json from the playback store and
// JSON-decodes it. Cached by the underlying store (S3/disk-page-cache).
func (s *Server) fetchMetafile(ctx context.Context, cid string) (*vod.Metafile, error) {
	key := vod.BlobsPrefix + cid + ".json"
	r, err := s.playbackStore.Open(ctx, key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "BlobNotFound")
		}
		log.Error(ctx, "playback: open metafile failed", "cid", cid, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer r.Close()
	body := make([]byte, r.Size())
	if _, err := r.ReadAt(body, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var meta vod.Metafile
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("malformed metafile for cid %s: %v", cid, err))
	}
	return &meta, nil
}

// --- playlist generation -----------------------------------------------

// blobURL is where the .m3u8 references segments + init blobs.
// Behaviour depends on whether a CDN sits in front of the bucket:
//
//   - cdnURL == "": self-hosted mode. The playlist links back to our
//     own getVideoBlob endpoint, content-addressed by query param.
//     The trailing `.m4s` is cosmetic but load-bearing: ffmpeg's HLS
//     demuxer refuses to fetch segment URLs whose extension isn't in
//     its `allowed_segment_extensions` list, and it checks the *end*
//     of the URL string. So `cid` (which carries the suffix) must be
//     the last query param — we can't let url.Values sort other keys
//     after it. The handler strips the suffix back off before lookup.
//
//   - cdnURL != "": CDN-fronted mode. The playlist links straight at
//     the configured CDN, which in turn serves blobs/<cid>.mp4 out of
//     the bucket. The `blobs/` prefix is baked in — operators point
//     --vod-cdn-url at the bucket root, and we know the layout from
//     vod.BlobsPrefix. We don't need the `.m4s` trick here because the
//     path itself carries `.mp4`; query params come after (browser HLS
//     players don't care).
//
// `did` is the owning account, carried in either mode so the request
// is attributable for egress accounting. `sid` is the playback session
// id used for view-count correlation; the CDN passes both through to
// access logs.
func blobURL(cdnURL, did, cid, sid string) string {
	q := url.Values{"did": {did}}
	if sid != "" {
		q.Set("sid", sid)
	}
	if cdnURL != "" {
		return strings.TrimRight(cdnURL, "/") + "/" + vod.BlobsPrefix +
			url.PathEscape(cid) + ".mp4?" + q.Encode()
	}
	return "/xrpc/place.stream.playback.getVideoBlob?" + q.Encode() +
		"&cid=" + url.QueryEscape(cid) + ".m4s"
}

// trackPlaylistURL is the URL to a single-track media playlist served
// by this same handler. `sid` is propagated from the master playlist
// so the player's media-playlist + segment requests all share an
// identifier.
func trackPlaylistURL(uri, track, sid string, startMS, endMS *int64) string {
	q := url.Values{"uri": {uri}, "track": {track}}
	if sid != "" {
		q.Set("sid", sid)
	}
	if startMS != nil {
		q.Set("start", strconv.FormatInt(*startMS, 10))
	}
	if endMS != nil {
		q.Set("end", strconv.FormatInt(*endMS, 10))
	}
	return "/xrpc/place.stream.playback.getVideoPlaylist?" + q.Encode()
}

// masterPlaylist emits an HLS multi-variant master, one EXT-X-STREAM-INF
// per video track and one EXT-X-MEDIA per audio track. Time-range
// params (if present) are propagated to the per-track media playlist
// URLs so the player keeps the clip bounds. `sid` is threaded into
// every sub-playlist URL so the per-track requests + downstream
// segment fetches share one playback-session identifier.
func masterPlaylist(meta *vod.Metafile, uri, sid string, startMS, endMS *int64) string {
	lines := []string{"#EXTM3U", "#EXT-X-VERSION:6", ""}

	// Collect audio tracks in deterministic order, pick a default
	// (prefer AAC since Safari is fussy about it).
	audioIDs := tracksOfType(meta, "audio")
	defaultAudio := pickDefaultAudio(meta, audioIDs)
	for _, tid := range audioIDs {
		t := meta.Tracks[tid]
		isDefault := tid == defaultAudio
		lines = append(lines, fmt.Sprintf(
			`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME=%q,DEFAULT=%s,AUTOSELECT=YES,CHANNELS=%q,URI=%q`,
			t.Codec,
			yesNo(isDefault),
			fmt.Sprintf("%d", channelsOrDefault(t.Channels)),
			trackPlaylistURL(uri, tid, sid, startMS, endMS),
		))
	}
	if len(audioIDs) > 0 {
		lines = append(lines, "")
	}

	// Compute an audio CODECS string to advertise alongside each video.
	// Prefer the default audio's codec; fall back to whatever's first.
	audioCodec := ""
	if defaultAudio != "" {
		audioCodec = meta.Tracks[defaultAudio].Codec
	} else if len(audioIDs) > 0 {
		audioCodec = meta.Tracks[audioIDs[0]].Codec
	}

	videoIDs := tracksOfType(meta, "video")
	for _, tid := range videoIDs {
		t := meta.Tracks[tid]
		bandwidth, frameRate := computeBandwidth(t)
		codecs := t.Codec
		if audioCodec != "" {
			codecs = t.Codec + "," + audioCodec
		}
		streamInf := fmt.Sprintf(
			`#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS=%q,RESOLUTION=%dx%d,FRAME-RATE=%.3f`,
			bandwidth, codecs, t.Width, t.Height, frameRate,
		)
		if audioCodec != "" {
			streamInf += `,AUDIO="audio"`
		}
		lines = append(lines, streamInf)
		lines = append(lines, trackPlaylistURL(uri, tid, sid, startMS, endMS))
	}

	return strings.Join(lines, "\n") + "\n"
}

// mediaPlaylist emits a single-track HLS playlist with one
// EXT-X-BYTERANGE per segment pointing at the blob host (CDN or local),
// plus an EXT-X-MAP for the per-track init segment. `ownerDID` is
// threaded into every blob URL for egress accounting and `sid` for
// view-count correlation against the request that built this playlist.
// `cdnURL` is the configured CDN root, or "" for self-hosted serving.
func mediaPlaylist(meta *vod.Metafile, trackID, ownerDID, sid, cdnURL string, startMS, endMS *int64) (string, error) {
	t, ok := meta.Tracks[trackID]
	if !ok {
		return "", echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}
	segments := filterSegments(t.Segments, t.Timescale, startMS, endMS)

	var maxDurSec float64
	for _, s := range segments {
		d := float64(s.DurationTicks) / float64(t.Timescale)
		if d > maxDurSec {
			maxDurSec = d
		}
	}
	targetDuration := int(maxDurSec)
	if maxDurSec > float64(targetDuration) {
		targetDuration++
	}
	if targetDuration < 1 {
		targetDuration = 1
	}

	lines := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:6",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXT-X-INDEPENDENT-SEGMENTS",
		fmt.Sprintf("#EXT-X-TARGETDURATION:%d", targetDuration),
		"#EXT-X-MEDIA-SEQUENCE:0",
		fmt.Sprintf(`#EXT-X-MAP:URI=%q`, blobURL(cdnURL, ownerDID, t.InitCID, sid)),
		"",
	}
	bURL := blobURL(cdnURL, ownerDID, t.BlobCID, sid)
	for _, seg := range segments {
		durSec := float64(seg.DurationTicks) / float64(t.Timescale)
		lines = append(lines,
			fmt.Sprintf("#EXTINF:%.6f,", durSec),
			fmt.Sprintf("#EXT-X-BYTERANGE:%d@%d", seg.Size, seg.Offset),
			bURL,
		)
	}
	lines = append(lines, "#EXT-X-ENDLIST")
	return strings.Join(lines, "\n") + "\n", nil
}

// composeClipBounds maps clip-local (queryStart, queryEnd) bounds and
// record-level (clipStartMS, clipEndMS) bounds into the parent
// video's timeline that filterSegments operates on. Either side can
// be nil/zero to mean "no bound on this end":
//
//   - clipStartMS == 0 + clipEndMS == nil: not a clip; the record is
//     a plain sourceTracks video. The returned bounds are just the
//     query params.
//
//   - For a clip, query bounds are translated by adding clipStartMS;
//     the end is clamped to clipEndMS so a player asking for more
//     than the clip can't escape it.
func composeClipBounds(clipStartMS int64, clipEndMS, queryStartMS, queryEndMS *int64) (*int64, *int64) {
	effStart := clipStartMS
	if queryStartMS != nil {
		effStart += *queryStartMS
	}

	var effEnd *int64
	if queryEndMS != nil {
		e := clipStartMS + *queryEndMS
		if clipEndMS != nil && e > *clipEndMS {
			e = *clipEndMS
		}
		effEnd = &e
	} else if clipEndMS != nil {
		e := *clipEndMS
		effEnd = &e
	}

	var startPtr *int64
	if effStart > 0 {
		startPtr = &effStart
	}
	return startPtr, effEnd
}

// filterSegments returns the subset of segments that overlap
// [startMS, endMS). Segments are GOP-aligned, so any segment whose
// time span intersects the requested range is included in full —
// no sub-segment splitting. Bounds are in the parent video's
// timeline; clip-record local times must be composed by the caller
// before they get here.
func filterSegments(segments []vod.MetafileSegment, timescale uint32, startMS, endMS *int64) []vod.MetafileSegment {
	if startMS == nil && endMS == nil {
		return segments
	}
	tsf := float64(timescale)
	startTicks := int64(0)
	endTicks := int64(1 << 62)
	if startMS != nil {
		startTicks = int64(float64(*startMS) / 1e3 * tsf)
	}
	if endMS != nil {
		endTicks = int64(float64(*endMS) / 1e3 * tsf)
	}

	out := segments[:0:0]
	cursor := int64(0)
	for _, seg := range segments {
		segEnd := cursor + int64(seg.DurationTicks)
		if segEnd > startTicks && cursor < endTicks {
			out = append(out, seg)
		}
		cursor = segEnd
	}
	return out
}

// computeBandwidth derives an approximate average bitrate (bits/s)
// and frame rate over all segments. Used as cosmetic HLS metadata —
// players use it to pick variants, but with only one variant it's
// purely informational. Returns (0, 0) for empty track.
func computeBandwidth(t vod.MetafileTrack) (bandwidth uint64, frameRate float64) {
	var totalBytes int64
	var totalTicks uint64
	var totalSamples uint32
	for _, s := range t.Segments {
		totalBytes += s.Size
		totalTicks += s.DurationTicks
		totalSamples += s.SampleCount
	}
	if totalTicks == 0 {
		return 0, 0
	}
	totalDur := float64(totalTicks) / float64(t.Timescale)
	if totalDur > 0 {
		bandwidth = uint64(float64(totalBytes*8) / totalDur)
		frameRate = float64(totalSamples) / totalDur
	}
	return bandwidth, frameRate
}

// tracksOfType returns the sorted track IDs of the given type.
func tracksOfType(meta *vod.Metafile, t string) []string {
	out := []string{}
	for id, tr := range meta.Tracks {
		if tr.Type == t {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// pickDefaultAudio prefers an AAC track (Safari requires it) and
// falls back to the first audio track. Returns "" if no audio.
func pickDefaultAudio(meta *vod.Metafile, audioIDs []string) string {
	for _, id := range audioIDs {
		if strings.HasPrefix(meta.Tracks[id].Codec, "mp4a") {
			return id
		}
	}
	if len(audioIDs) > 0 {
		return audioIDs[0]
	}
	return ""
}

func channelsOrDefault(c uint32) int {
	if c == 0 {
		return 2
	}
	return int(c)
}

func yesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}
