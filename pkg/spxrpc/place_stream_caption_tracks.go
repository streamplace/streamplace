package spxrpc

import (
	"context"
	"io"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/livecue"
	"stream.place/streamplace/pkg/captions/webvtt"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// captionSource is where a caption request's cues come from: a streamer's
// live window or a video.
type captionSource struct {
	streamer string // resolved DID, live
	video    string // normalized AT-URI, video
}

// resolveCaptionSource validates the streamer-or-video parameters of a
// request and applies the same availability rules as playback: an unpublished
// live window, a ban on the streamer's account, or a label or ban on the video
// hides it.
func (s *Server) resolveCaptionSource(ctx context.Context, streamer, video string) (captionSource, error) {
	switch {
	case streamer == "" && video == "":
		return captionSource{}, echo.NewHTTPError(http.StatusBadRequest, "one of streamer or video is required")
	case streamer != "" && video != "":
		return captionSource{}, echo.NewHTTPError(http.StatusBadRequest, "streamer and video are mutually exclusive")
	case streamer != "":
		did, err := s.resolveStreamer(ctx, streamer)
		if err != nil {
			return captionSource{}, err
		}
		if banned, err := s.accountBanned(did); err != nil {
			return captionSource{}, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		} else if banned {
			return captionSource{}, echo.NewHTTPError(http.StatusForbidden, "StreamUnavailable")
		}
		if s.mm == nil || !s.mm.LiveWindowPublished(did) {
			return captionSource{}, echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
		}
		return captionSource{streamer: did}, nil
	}
	aturi, err := s.normalizeURI(ctx, video)
	if err != nil {
		return captionSource{}, echo.NewHTTPError(http.StatusBadRequest, "video must be a valid AT-URI: "+err.Error())
	}
	if aturi.Collection() != videoCollection {
		return captionSource{}, echo.NewHTTPError(http.StatusBadRequest, "UnsupportedCollection: "+aturi.Collection().String())
	}
	uri := aturi.String()
	if labeled, err := s.recordLabeled(uri); err != nil {
		return captionSource{}, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if labeled {
		return captionSource{}, echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}
	if banned, err := s.accountBanned(aturi.Authority().String()); err != nil {
		return captionSource{}, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return captionSource{}, echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}
	if _, err := s.loadVideoRecord(ctx, uri); err != nil {
		return captionSource{}, err
	}
	return captionSource{video: uri}, nil
}

// tracks lists the caption tracks of the source.
func (s *Server) captionTracks(ctx context.Context, src captionSource) ([]captions.Track, error) {
	if src.streamer != "" {
		if hub := s.captionHub(); hub != nil {
			return hub.Tracks(src.streamer), nil
		}
		return nil, nil
	}
	if s.VideoCaptions == nil {
		return nil, nil
	}
	tracks, err := s.VideoCaptions.Tracks(ctx, src.video)
	if err != nil {
		log.Error(ctx, "captions: video tracks lookup failed", "video", src.video, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return tracks, nil
}

// handlePlaceStreamCaptionListTracks lists a live stream's caption tracks on
// this node, or a video's.
func (s *Server) handlePlaceStreamCaptionListTracks(ctx context.Context, streamer string, video string) (*placestream.CaptionListTracks_Output, error) {
	src, err := s.resolveCaptionSource(ctx, streamer, video)
	if err != nil {
		return nil, err
	}
	tracks, err := s.captionTracks(ctx, src)
	if err != nil {
		return nil, err
	}
	out := &placestream.CaptionListTracks_Output{Tracks: make([]placestream.CaptionDefs_TrackView, 0, len(tracks))}
	for _, t := range tracks {
		out.Tracks = append(out.Tracks, livecue.TrackView(t))
	}
	return out, nil
}

// handlePlaceStreamCaptionGetCaptions is the stub for the generated wrapper,
// which fixes the response's Content-Type; NewServer routes getCaptions to
// HandleGetCaptions instead.
func (s *Server) handlePlaceStreamCaptionGetCaptions(ctx context.Context, end int, format string, mpegts int, start int, streamer string, track string, video string) (io.Reader, error) {
	return nil, stubMisrouted("getCaptions")
}

// farFuture bounds "every cue" queries of the live hub.
var farFuture = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// HandleGetCaptions serves one caption track as WebVTT, SRT, or JSON.
//
// Live: the final cues in the node's current window, with times counted from
// the live window's epoch (the start of its first segment, so the times line
// up with the HLS subtitle segments), or from the first cue when the stream
// has no window. Video: the track's cues, counted from the video start.
// start and end (ms from the video start) narrow a video's cues to those
// overlapping the range, and mpegts adds the X-TIMESTAMP-MAP header, which is
// how HLS subtitle segments are fetched (see vodCaptionSegmentURL). Plain
// downloads carry a Content-Disposition attachment filename.
func (s *Server) HandleGetCaptions(c echo.Context) error {
	ctx := c.Request().Context()
	// The cosmetic ".vtt" of HLS subtitle segment URLs, for ffmpeg-based
	// players' segment extension allowlist.
	trackID := strings.TrimSuffix(c.QueryParam("track"), ".vtt")
	if trackID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "track is required")
	}
	format := c.QueryParam("format")
	if format == "" {
		format = "vtt"
	}
	if format != "vtt" && format != "srt" && format != "json" {
		return echo.NewHTTPError(http.StatusBadRequest, "format must be vtt, srt, or json")
	}
	startMS, err := optionalInt64Param(c, "start")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	endMS, err := optionalInt64Param(c, "end")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	mpegts, err := optionalInt64Param(c, "mpegts")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if mpegts != nil && *mpegts < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "mpegts must not be negative")
	}

	src, err := s.resolveCaptionSource(ctx, c.QueryParam("streamer"), c.QueryParam("video"))
	if err != nil {
		return err
	}
	var cues []webvtt.Cue
	var epoch time.Time
	var filename string
	if src.streamer != "" {
		cues, epoch, err = s.liveCaptionCues(src.streamer, trackID)
		filename = src.streamer + "-" + trackID
	} else {
		cues, err = s.videoCaptionCues(ctx, src.video, trackID, startMS, endMS)
		filename = path.Base(src.video) + "-" + trackID
	}
	if err != nil {
		return err
	}

	var body []byte
	var contentType string
	switch format {
	case "vtt":
		var tm *webvtt.TimestampMap
		if mpegts != nil {
			tm = &webvtt.TimestampMap{MPEGTS: uint64(*mpegts)}
		}
		body, contentType = webvtt.EncodeVTT(cues, tm), "text/vtt; charset=utf-8"
	case "srt":
		body, contentType = webvtt.EncodeSRT(cues), "application/x-subrip; charset=utf-8"
	case "json":
		body, err = webvtt.EncodeJSON(cues, epoch)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		contentType = "application/json"
	}

	h := c.Response().Header()
	if startMS == nil && endMS == nil && mpegts == nil {
		if disp := mime.FormatMediaType("attachment", map[string]string{"filename": captionFilename(filename, format)}); disp != "" {
			h.Set("Content-Disposition", disp)
		}
	}
	if src.streamer != "" {
		h.Set("Cache-Control", "no-cache")
	} else {
		h.Set("Cache-Control", "public, max-age=60")
	}
	return c.Blob(http.StatusOK, contentType, body)
}

// liveCaptionCues returns a live track's final cues as offsets from the live
// window's epoch, and that epoch.
func (s *Server) liveCaptionCues(did, trackID string) ([]webvtt.Cue, time.Time, error) {
	if _, ok := s.liveCaptionTrack(did, trackID); !ok {
		return nil, time.Time{}, echo.NewHTTPError(http.StatusNotFound, "NotFound")
	}
	live := s.captionHub().Cues(did, trackID, time.Time{}, farFuture)
	epoch := s.liveWindowEpoch(did)
	if epoch.IsZero() && len(live) > 0 {
		epoch = live[0].Start
	}
	cues := make([]webvtt.Cue, 0, len(live))
	for _, cue := range live {
		start := max(cue.Start.Sub(epoch), 0)
		cues = append(cues, webvtt.Cue{ID: cue.ID, Start: start, End: max(cue.End.Sub(epoch), start), Text: cue.Text})
	}
	return cues, epoch, nil
}

// liveWindowEpoch is the zero of the live HLS subtitle timeline, or the zero
// time when the streamer has no live window on this node.
func (s *Server) liveWindowEpoch(did string) time.Time {
	if s.mm == nil {
		return time.Time{}
	}
	if w := s.mm.GetLiveWindow(did); w != nil {
		return w.SubtitleEpoch()
	}
	return time.Time{}
}

// videoCaptionCues returns a video track's cues, those overlapping
// [startMS, endMS) when given.
func (s *Server) videoCaptionCues(ctx context.Context, video, trackID string, startMS, endMS *int64) ([]webvtt.Cue, error) {
	tracks, err := s.captionTracks(ctx, captionSource{video: video})
	if err != nil {
		return nil, err
	}
	found := false
	for _, t := range tracks {
		found = found || t.ID == trackID
	}
	if !found {
		return nil, echo.NewHTTPError(http.StatusNotFound, "NotFound")
	}
	timed, err := s.VideoCaptions.Cues(ctx, video, trackID)
	if err != nil {
		log.Error(ctx, "captions: video cues lookup failed", "video", video, "track", trackID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	var from, to time.Duration = 0, 1<<63 - 1
	if startMS != nil {
		from = time.Duration(*startMS) * time.Millisecond
	}
	if endMS != nil {
		to = time.Duration(*endMS) * time.Millisecond
	}
	cues := make([]webvtt.Cue, 0, len(timed))
	for _, cue := range timed {
		if cue.End <= from || cue.Start >= to {
			continue
		}
		cues = append(cues, webvtt.Cue{ID: cue.ID, Start: cue.Start, End: cue.End, Text: cue.Text})
	}
	return cues, nil
}

var filenameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// captionFilename is a safe download name for a caption track.
func captionFilename(trackID, format string) string {
	name := strings.Trim(filenameUnsafe.ReplaceAllString(trackID, "-"), "-.")
	if name == "" {
		name = "captions"
	}
	return name + "." + format
}
