package spxrpc

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// replayIdleWindow bounds the chat of a livestream record that was never
// ended: messages up to this long after the last heartbeat count.
const replayIdleWindow = 30 * time.Minute

// handlePlaceStreamChatGetReplay serves the chat of the livestream(s) a
// video is the recording of. The video's connections name the livestream
// records (the app's Livestreams tab and the operator's finalize both
// connect a recording to them); only records of the video's own author
// count, so a video cannot borrow another channel's chat. The messages are
// the streamer's chat from the first record's start to the last record's
// end, oldest first, with the recording's start time for the player to line
// them up against the video.
func (s *Server) handlePlaceStreamChatGetReplay(ctx context.Context, cursor string, limit int, video string) (*placestream.ChatGetReplay_Output, error) {
	if video == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "video is required")
	}
	aturi, err := s.normalizeURI(ctx, video)
	if err != nil {
		return nil, err
	}
	rec, err := s.model.GetVideoByURI(ctx, aturi.String())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if rec == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "VideoNotFound")
	}
	author := aturi.Authority().String()
	out := &placestream.ChatGetReplay_Output{Messages: []placestream.ChatDefs_MessageView{}, Livestreams: []string{}}
	uris := livestreamConnections(rec, author)
	if len(uris) == 0 {
		return out, nil
	}

	// The window: from the earliest record's start to the latest record's
	// end, in record order.
	type span struct {
		uri        string
		start, end time.Time
	}
	var spans []span
	for _, u := range uris {
		ls, err := s.model.GetLivestream(u)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if ls == nil || ls.Livestream == nil {
			continue
		}
		lrec := &placestream.Livestream{}
		if err := glex.DecodeCBOR(*ls.Livestream, lrec); err != nil {
			log.Warn(ctx, "chat replay: undecodable livestream record", "uri", u, "error", err)
			continue
		}
		start, err := time.Parse(time.RFC3339, lrec.CreatedAt)
		if err != nil {
			start = ls.CreatedAt
		}
		end := start.Add(replayIdleWindow)
		switch {
		case lrec.EndedAt != nil:
			if t, err := time.Parse(time.RFC3339, *lrec.EndedAt); err == nil {
				end = t
			}
		case lrec.LastSeenAt != nil:
			if t, err := time.Parse(time.RFC3339, *lrec.LastSeenAt); err == nil {
				end = t.Add(replayIdleWindow)
			}
		}
		spans = append(spans, span{uri: u, start: start, end: end})
	}
	if len(spans) == 0 {
		return out, nil
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
	from, to := spans[0].start, spans[0].end
	for _, sp := range spans {
		out.Livestreams = append(out.Livestreams, sp.uri)
		if sp.end.After(to) {
			to = sp.end
		}
	}

	// The recording's first moment: the first recorded object's start when
	// this node has the recording, else the first record's creation.
	startedAt := from
	if s.statefulDB != nil {
		if segs, err := s.statefulDB.ListS3SegmentsForLivestreams(ctx, out.Livestreams); err == nil && len(segs) > 0 && !segs[0].StartedAt.IsZero() {
			startedAt = segs[0].StartedAt
		}
	}
	st := startedAt.UTC().Format(time.RFC3339Nano)
	out.StartedAt = &st

	msgs, next, err := s.model.ChatMessagesBetween(ctx, author, from, to, cursor, limit)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	out.Messages = msgs
	if next != "" {
		out.Cursor = &next
	}
	return out, nil
}
