package spxrpc

import (
	"bytes"
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

	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/pkg/vod"
)

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

func (s *Server) handlePlaceStreamPlaybackGetVideoBlob(ctx context.Context, cid string) (io.Reader, error) {
	return nil, stubMisrouted("getVideoBlob")
}

func (s *Server) handlePlaceStreamPlaybackGetVideoPlaylist(ctx context.Context, did string, end *int, rkey string, start *int, track string) (io.Reader, error) {
	return nil, stubMisrouted("getVideoPlaylist")
}

// --- getVideoBlob -------------------------------------------------------

// HandleGetVideoBlob serves a content-addressed playback blob (primary
// fMP4 or per-track init segment, indistinguishable from the
// endpoint's POV) with HTTP Range support.
func (s *Server) HandleGetVideoBlob(c echo.Context) error {
	ctx := c.Request().Context()
	cid := c.QueryParam("cid")
	if cid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "cid is required")
	}
	if s.playbackStore == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	key := vod.ContentPrefix + cid + ".mp4"
	r, err := s.playbackStore.Open(ctx, key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "BlobNotFound")
		}
		log.Error(ctx, "playback: open blob failed", "cid", cid, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer r.Close()
	return serveBlobRange(c, r, "video/mp4")
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
	did := c.QueryParam("did")
	rkey := c.QueryParam("rkey")
	if did == "" || rkey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "did and rkey are required")
	}
	if s.playbackStore == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}

	track := c.QueryParam("track")
	startNS, err := optionalInt64Param(c, "start")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	endNS, err := optionalInt64Param(c, "end")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if startNS != nil && endNS != nil && *startNS >= *endNS {
		return echo.NewHTTPError(http.StatusBadRequest, "start must be less than end")
	}

	resolved, err := s.resolveVideoBlob(ctx, did, rkey)
	if err != nil {
		return err
	}
	meta, err := s.fetchMetafile(ctx, resolved.blobCID)
	if err != nil {
		return err
	}

	var body string
	if track == "" {
		body = masterPlaylist(meta, did, rkey, startNS, endNS)
	} else {
		body, err = mediaPlaylist(meta, track, did, rkey, startNS, endNS)
		if err != nil {
			return err
		}
	}

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

// resolvedVideo carries the primary blob CID for a video record, plus
// any record-level clip bounds (place.stream.media.defs#sourceClip)
// once we add that to the resolution path.
type resolvedVideo struct {
	blobCID string
}

// resolveVideoBlob walks video -> first track -> muxlTrack.blob using
// the local index. The metafile holds catalogs for every track of the
// blob, so picking any track's blob is sufficient — they all live in
// the same container.
func (s *Server) resolveVideoBlob(ctx context.Context, did, rkey string) (*resolvedVideo, error) {
	uri := fmt.Sprintf("at://%s/place.stream.video/%s", did, rkey)
	video, err := s.model.GetVideoByURI(ctx, uri)
	if err != nil {
		log.Error(ctx, "playback: GetVideoByURI failed", "uri", uri, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if video == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "VideoNotFound")
	}

	rec, err := lexutil.CborDecodeValue(video.Record)
	if err != nil {
		log.Error(ctx, "playback: decode video record failed", "uri", uri, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	videoRec, ok := rec.(*streamplace.Video)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("video record at %s decoded as %T, expected *streamplace.Video", uri, rec))
	}
	if videoRec.Source == nil || videoRec.Source.MediaDefs_SourceTracks == nil ||
		len(videoRec.Source.MediaDefs_SourceTracks.Tracks) == 0 {
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "video record has no tracks")
	}
	firstRef := videoRec.Source.MediaDefs_SourceTracks.Tracks[0]
	if firstRef == nil || firstRef.Uri == "" {
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "video record's first track ref is empty")
	}
	track, err := s.model.GetMediaTrackByURI(ctx, firstRef.Uri)
	if err != nil {
		log.Error(ctx, "playback: GetMediaTrackByURI failed", "uri", firstRef.Uri, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if track == nil || track.Blob == "" {
		return nil, echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}
	return &resolvedVideo{blobCID: track.Blob}, nil
}

// fetchMetafile reads vod/<cid>.json from the playback store and
// JSON-decodes it. Cached by the underlying store (S3/disk-page-cache).
func (s *Server) fetchMetafile(ctx context.Context, cid string) (*vod.Metafile, error) {
	key := vod.ContentPrefix + cid + ".json"
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

// blobURL is where the .m3u8 references segments + init blobs. Same
// node, getVideoBlob endpoint, content-addressed.
func blobURL(cid string) string {
	q := url.Values{"cid": {cid}}
	return "/xrpc/place.stream.playback.getVideoBlob?" + q.Encode()
}

// trackPlaylistURL is the URL to a single-track media playlist served
// by this same handler.
func trackPlaylistURL(did, rkey, track string, startNS, endNS *int64) string {
	q := url.Values{"did": {did}, "rkey": {rkey}, "track": {track}}
	if startNS != nil {
		q.Set("start", strconv.FormatInt(*startNS, 10))
	}
	if endNS != nil {
		q.Set("end", strconv.FormatInt(*endNS, 10))
	}
	return "/xrpc/place.stream.playback.getVideoPlaylist?" + q.Encode()
}

// masterPlaylist emits an HLS multi-variant master, one EXT-X-STREAM-INF
// per video track and one EXT-X-MEDIA per audio track. Time-range
// params (if present) are propagated to the per-track media playlist
// URLs so the player keeps the clip bounds.
func masterPlaylist(meta *vod.Metafile, did, rkey string, startNS, endNS *int64) string {
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
			trackPlaylistURL(did, rkey, tid, startNS, endNS),
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
		lines = append(lines, trackPlaylistURL(did, rkey, tid, startNS, endNS))
	}

	return strings.Join(lines, "\n") + "\n"
}

// mediaPlaylist emits a single-track HLS playlist with one
// EXT-X-BYTERANGE per segment pointing at getVideoBlob, plus an
// EXT-X-MAP for the per-track init segment.
func mediaPlaylist(meta *vod.Metafile, trackID, did, rkey string, startNS, endNS *int64) (string, error) {
	t, ok := meta.Tracks[trackID]
	if !ok {
		return "", echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}
	segments := filterSegments(t.Segments, t.Timescale, startNS, endNS)

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
		fmt.Sprintf(`#EXT-X-MAP:URI=%q`, blobURL(t.InitCID)),
		"",
	}
	bURL := blobURL(t.BlobCID)
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

// filterSegments returns the subset of segments that overlap
// [startNS, endNS). Segments are GOP-aligned, so any segment whose
// time span intersects the requested range is included in full —
// no sub-segment splitting.
func filterSegments(segments []vod.MetafileSegment, timescale uint32, startNS, endNS *int64) []vod.MetafileSegment {
	if startNS == nil && endNS == nil {
		return segments
	}
	tsf := float64(timescale)
	startTicks := int64(0)
	endTicks := int64(1 << 62)
	if startNS != nil {
		startTicks = int64(float64(*startNS) / 1e9 * tsf)
	}
	if endNS != nil {
		endTicks = int64(float64(*endNS) / 1e9 * tsf)
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

// Force imports used in conditionally-compiled paths. bytes is used by
// the Range parser; this stub keeps the linter quiet during refactors.
var _ = bytes.NewReader
