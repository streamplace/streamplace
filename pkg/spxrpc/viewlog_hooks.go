package spxrpc

import (
	"time"

	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/viewlog"
)

// logManifestRequest captures a playlist fetch (master or media) for
// later view-count aggregation. No-op when --view-log-flush-interval=0
// has left s.viewLog nil.
func (s *Server) logManifestRequest(c echo.Context, uri, sid, track, kind string) {
	if s.viewLog == nil {
		return
	}
	ctx := c.Request().Context()
	now := time.Now().UTC()
	ipHash, err := s.viewLog.Salts().HashIP(c.RealIP(), now)
	if err != nil {
		// Drop the hash but still log the event — better a partial
		// record than a missed view.
		log.Error(ctx, "viewlog: hash IP", "error", err)
		ipHash = ""
	}
	s.viewLog.Log(ctx, viewlog.Event{
		Ts:           now,
		Type:         viewlog.EventTypeManifestRequest,
		VideoURI:     uri,
		SID:          sid,
		IPHash:       ipHash,
		ManifestKind: kind,
		Track:        track,
	})
}

// logSegmentRequest captures a content-addressed blob fetch. The
// aggregator joins CID → place.stream.video.AT-URI via the MediaTrack
// index (one row per (track, blob)); we log raw CID + owner here so
// the handler doesn't pay a DB roundtrip on the hot path.
func (s *Server) logSegmentRequest(c echo.Context, cid, ownerDID, sid string, rangeStart, rangeEnd int64) {
	if s.viewLog == nil {
		return
	}
	ctx := c.Request().Context()
	now := time.Now().UTC()
	ipHash, err := s.viewLog.Salts().HashIP(c.RealIP(), now)
	if err != nil {
		log.Error(ctx, "viewlog: hash IP", "error", err)
		ipHash = ""
	}
	s.viewLog.Log(ctx, viewlog.Event{
		Ts:         now,
		Type:       viewlog.EventTypeSegmentRequest,
		SID:        sid,
		IPHash:     ipHash,
		CID:        cid,
		OwnerDID:   ownerDID,
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
	})
}
