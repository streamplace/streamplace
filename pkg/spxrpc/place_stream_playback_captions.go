package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/psession"
	"stream.place/streamplace/pkg/vod"
)

// captionRecognitionLag is how far behind the audio the recognizer's final
// cues run, over and above the mastering hold (cli.CaptionsMasterDelay): the
// time a live subtitle segment waits before it is listed with whatever cues
// exist for it, in case no later cue shows the track has caught up. A
// segment is listed earlier whenever the track has published final cues past
// its end.
const captionRecognitionLag = 4 * time.Second

// captionBudget is the live subtitle playlist's lag behind the video: how
// old a video segment must be before its captions are served as complete.
func (s *Server) captionBudget() time.Duration {
	return s.cli.CaptionsMasterDelay + captionRecognitionLag
}

// captionHub is this node's live caption hub, or nil when there is none.
func (s *Server) captionHub() *captions.Hub {
	if s.bus == nil {
		return nil
	}
	return s.bus.Captions
}

// liveCaptionTrack finds one of a streamer's live caption tracks.
func (s *Server) liveCaptionTrack(did, id string) (captions.Track, bool) {
	hub := s.captionHub()
	if hub == nil {
		return captions.Track{}, false
	}
	for _, t := range hub.Tracks(did) {
		if t.ID == id {
			return t, true
		}
	}
	return captions.Track{}, false
}

// captionsFinal is the "the track has caught up" test of the live subtitle
// window for one track: it has published final cues past the end of a
// segment, so nothing more will be added to the segment (see
// livehls.Writer.SubtitleWindow).
func (s *Server) captionsFinal(did, trackID string) func(start, end time.Time) bool {
	hub := s.captionHub()
	budget := s.captionBudget()
	return func(_, end time.Time) bool {
		return len(hub.Cues(did, trackID, end, end.Add(budget))) > 0
	}
}

// captionRenditionName is a rendition's NAME: the track's label, or its
// language and source. Names must be unique within the group.
func captionRenditionName(t captions.Track, taken map[string]bool) string {
	name := t.Label
	if name == "" {
		name = fmt.Sprintf("%s (%s)", t.Language, t.Source)
	}
	if taken[name] {
		name = fmt.Sprintf("%s (%s)", name, t.ID)
	}
	taken[name] = true
	return name
}

// --- live ---------------------------------------------------------------

// liveSubtitleRenditions lists the master playlist's caption renditions: the
// streamer's caption tracks that have subtitle segments ready right now. The
// master playlist is fetched once at the start of playback by most players,
// so a track that appears later (the recognizer starts after the first
// words, or a node adds a language) is only offered to players that fetch
// the master afterwards.
func (s *Server) liveSubtitleRenditions(w *livehls.Writer, did, sid string, nocdn bool) []livehls.SubtitleRendition {
	hub := s.captionHub()
	if hub == nil {
		return nil
	}
	var out []livehls.SubtitleRendition
	taken := map[string]bool{}
	for _, t := range hub.Tracks(did) {
		if len(w.SubtitleWindow(s.captionBudget(), s.captionsFinal(did, t.ID)).Segments) == 0 {
			continue
		}
		out = append(out, livehls.SubtitleRendition{
			Name:     captionRenditionName(t, taken),
			Language: t.Language,
			URI:      liveCaptionPlaylistURL(did, t.ID, sid, nocdn),
		})
	}
	return out
}

// liveSubtitlePlaylist renders a caption track's subtitle media playlist, or
// "" when the track is unknown or has nothing ready.
func (s *Server) liveSubtitlePlaylist(w *livehls.Writer, did, trackID, sid string) string {
	if _, ok := s.liveCaptionTrack(did, trackID); !ok {
		return ""
	}
	win := w.SubtitleWindow(s.captionBudget(), s.captionsFinal(did, trackID))
	if len(win.Segments) == 0 {
		return ""
	}
	return win.Playlist(func(seq uint64) string {
		return liveCaptionSegmentURL(did, trackID, seq, sid)
	})
}

// serveLiveSubtitleSegment serves one WebVTT subtitle segment of a caption
// track: the segment of the video track's window with the same media-sequence
// number, holding the track's final cues that overlap it. Same session and
// pre-live rules as the video segments.
func (s *Server) serveLiveSubtitleSegment(c echo.Context, streamer, trackID, segParam, sid string) error {
	ctx := c.Request().Context()
	did, err := s.resolveStreamer(ctx, streamer)
	if err != nil {
		return err
	}
	// The cosmetic ".vtt" lets ffmpeg-based players fetch the URL; ignore it.
	seq, err := strconv.ParseUint(strings.TrimSuffix(segParam, ".vtt"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "seg must be a media-sequence number")
	}
	w := s.mm.GetLiveWindow(did)
	if w == nil {
		return echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
	}
	sess, err := s.verifySession(ctx, sid, did)
	if err != nil {
		return err
	}
	if !s.mm.LiveWindowPublished(did) && !s.cli.WideOpen && sess.Scope != psession.ScopeOwner {
		return echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
	}
	s.mm.TouchHLSSession(did, sess.ID)

	if _, ok := s.liveCaptionTrack(did, trackID); !ok {
		return echo.NewHTTPError(http.StatusNotFound, "CaptionTrackNotFound")
	}
	seg, epoch, ok := w.SubtitleSegment(seq, s.captionBudget(), s.captionsFinal(did, trackID))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "SegmentNotFound")
	}
	body := livehls.SegmentVTT(seg, epoch, s.captionHub().Cues(did, trackID, seg.Start, seg.End))
	// Late cues can still land in a segment that went out on the latency
	// budget, so unlike a video segment it is not immutable.
	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.Blob(http.StatusOK, "text/vtt; charset=utf-8", body)
}

// liveCaptionPlaylistURL is the URL of a caption track's subtitle playlist,
// served by getLivePlaylist.
func liveCaptionPlaylistURL(did, trackID, sid string, nocdn bool) string {
	q := withSID(url.Values{"streamer": {did}, "captions": {trackID}}, sid)
	if nocdn {
		q.Set("nocdn", "1")
	}
	return "/xrpc/place.stream.playback.getLivePlaylist?" + q.Encode()
}

// liveCaptionSegmentURL is the URL of one subtitle segment, served by
// getLiveSegment. Subtitle segments are small and per-session, so they always
// come from the node, never the live CDN. The ".vtt" comes last, for ffmpeg's
// segment extension allowlist; the handler strips it.
func liveCaptionSegmentURL(did, trackID string, seq uint64, sid string) string {
	q := withSID(url.Values{"streamer": {did}, "captions": {trackID}}, sid)
	return "/xrpc/place.stream.playback.getLiveSegment?" + q.Encode() +
		"&seg=" + strconv.FormatUint(seq, 10) + ".vtt"
}

// --- VOD ----------------------------------------------------------------

// vodSubtitleRenditions lists the caption renditions of a video's master
// playlist. Caption lookup trouble is logged and leaves the video playable
// without captions.
func (s *Server) vodSubtitleRenditions(ctx context.Context, uri, sid string, startMS, endMS *int64) []livehls.SubtitleRendition {
	if s.VideoCaptions == nil {
		return nil
	}
	tracks, err := s.VideoCaptions.Tracks(ctx, uri)
	if err != nil {
		log.Error(ctx, "playback: caption tracks lookup failed", "uri", uri, "error", err)
		return nil
	}
	var out []livehls.SubtitleRendition
	taken := map[string]bool{}
	for _, t := range tracks {
		out = append(out, livehls.SubtitleRendition{
			Name:     captionRenditionName(t, taken),
			Language: t.Language,
			URI:      vodCaptionPlaylistURL(uri, t.ID, sid, startMS, endMS),
		})
	}
	return out
}

// vodCaptionPlaylistURL is the URL of a caption track's subtitle playlist,
// served by getVideoPlaylist.
func vodCaptionPlaylistURL(uri, trackID, sid string, startMS, endMS *int64) string {
	q := url.Values{"uri": {uri}, "captions": {trackID}}
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

// vodCaptionSegmentURL is the URL of one VOD subtitle segment: getCaptions
// over the segment's time range, with the X-TIMESTAMP-MAP that places it on
// the video's media timeline. The ".vtt" on the track comes last, for ffmpeg's
// segment extension allowlist; getCaptions strips it.
func vodCaptionSegmentURL(uri, trackID string, seg livehls.SubtitleSegment, epoch time.Time) string {
	q := url.Values{
		"video":  {uri},
		"format": {"vtt"},
		"start":  {strconv.FormatInt(max(seg.Start.Sub(epoch), 0).Milliseconds(), 10)},
		// Rounded up, so a cue ending inside the last millisecond is kept.
		"end":    {strconv.FormatInt((max(seg.End.Sub(epoch), 0) + time.Millisecond - 1).Milliseconds(), 10)},
		"mpegts": {strconv.FormatUint(seg.MPEGTS, 10)},
	}
	return "/xrpc/place.stream.caption.getCaptions?" + q.Encode() + "&track=" + url.QueryEscape(trackID) + ".vtt"
}

// vodSubtitlePlaylist renders a caption track's subtitle playlist for a
// video: one WebVTT segment per segment of the video's reference track
// (its first video track, else its first audio track) that the video
// playlist would list, so the two line up.
func (s *Server) vodSubtitlePlaylist(ctx context.Context, meta *vod.Metafile, uri, trackID string, clipStartMS int64, startMS, endMS *int64) (string, error) {
	if s.VideoCaptions == nil {
		return "", echo.NewHTTPError(http.StatusNotFound, "CaptionTrackNotFound")
	}
	tracks, err := s.VideoCaptions.Tracks(ctx, uri)
	if err != nil {
		log.Error(ctx, "playback: caption tracks lookup failed", "uri", uri, "error", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	found := false
	for _, t := range tracks {
		found = found || t.ID == trackID
	}
	if !found {
		return "", echo.NewHTTPError(http.StatusNotFound, "CaptionTrackNotFound")
	}

	refID := ""
	if ids := tracksOfType(meta, "video"); len(ids) > 0 {
		refID = ids[0]
	} else if ids := tracksOfType(meta, "audio"); len(ids) > 0 {
		refID = ids[0]
	}
	ref, ok := meta.Tracks[refID]
	if !ok || ref.Timescale == 0 {
		return "", echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
	}

	// The segments the video playlist lists: filterSegments' result is a
	// contiguous run of the track's segments, found again by byte offset.
	listed, discSeq := filterSegments(ref.Segments, ref.Timescale, startMS, endMS)
	if len(listed) == 0 {
		return "", echo.NewHTTPError(http.StatusNotFound, "CaptionTrackNotFound")
	}
	first := 0
	for first < len(ref.Segments) && ref.Segments[first].Offset != listed[0].Offset {
		first++
	}
	last := first + len(listed) - 1

	durations := make([]uint64, len(ref.Segments))
	discontinuity := make([]bool, len(ref.Segments))
	for i, seg := range ref.Segments {
		durations[i] = seg.DurationTicks
		discontinuity[i] = seg.Discontinuity
	}

	var store blob.Reader
	if s.playbackStore != nil {
		store, err = s.playbackStore.Open(ctx, vod.BlobsPrefix+ref.BlobCID+".mp4")
		if err != nil {
			log.Warn(ctx, "playback: open blob for caption timing failed", "cid", ref.BlobCID, "error", err)
			store = nil
		} else {
			defer store.Close()
		}
	}
	tfdt := func(i int) (uint64, bool) {
		if store == nil {
			return 0, false
		}
		seg := ref.Segments[i]
		v, ok := livehls.ReadFirstTFDT(store, seg.Offset+meta.FlatHeaderSize, seg.Size)
		if !ok {
			log.Warn(ctx, "playback: no tfdt for caption timing", "cid", ref.BlobCID, "segment", i)
		}
		return v, ok
	}

	win := livehls.VODSubtitleWindow(ref.Timescale, durations, discontinuity, first, last,
		time.Duration(clipStartMS)*time.Millisecond, discSeq, tfdt)
	return win.Playlist(func(seq uint64) string {
		return vodCaptionSegmentURL(uri, trackID, win.Segments[seq], win.Epoch)
	}), nil
}
