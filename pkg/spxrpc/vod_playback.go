package spxrpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/psession"
	"stream.place/streamplace/pkg/vod"
)

type videoPlayback struct {
	uri, did                         string
	session                          playbackSession
	resolved                         *resolvedVideo
	meta                             *vod.Metafile
	queryStart, queryEnd, start, end *int64
}

func (s *Server) gateVideo(uri, did string) error {
	if labeled, err := s.recordLabeled(uri); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if labeled {
		return echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}
	if banned, err := s.accountBanned(did); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return echo.NewHTTPError(http.StatusForbidden, "VideoUnavailable")
	}
	return nil
}

func (s *Server) resolvePlaybackRequest(c echo.Context, redirect bool) (*videoPlayback, error) {
	ctx := c.Request().Context()
	rawURI := c.QueryParam("uri")
	if rawURI == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri is required")
	}
	aturi, err := s.normalizeURI(ctx, rawURI)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uri must be a valid AT-URI: "+err.Error())
	}
	if aturi.Collection() != videoCollection {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("UnsupportedCollection: %s", aturi.Collection()))
	}
	if s.playbackStore == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	start, err := optionalInt64Param(c, "start")
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	end, err := optionalInt64Param(c, "end")
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if start != nil && end != nil && *start >= *end {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "start must be less than end")
	}
	sid := c.QueryParam("sid")
	if sid != "" && !psession.WellFormed(sid) {
		return nil, errBadSession
	}
	uri, did := aturi.String(), aturi.Authority().String()
	if err := s.gateVideo(uri, did); err != nil {
		return nil, err
	}
	session, err := s.resolveSession(ctx, sid, did, sessionClient(c), redirect)
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolveVideoBlob(ctx, uri)
	if err != nil {
		return nil, err
	}
	meta, err := s.cachedMetafile(ctx, resolved.blobCID)
	if err != nil {
		return nil, err
	}
	effectiveStart, effectiveEnd := composeClipBounds(resolved.clipStartMS, resolved.clipEndMS, start, end)
	return &videoPlayback{uri: uri, did: did, session: session, resolved: resolved, meta: meta, queryStart: start, queryEnd: end, start: effectiveStart, end: effectiveEnd}, nil
}

func (s *Server) ResolveServableVideo(ctx context.Context, did, rkey, sid string) (*vod.ServableVideo, error) {
	aturi, err := s.normalizeURI(ctx, fmt.Sprintf("at://%s/%s/%s", did, videoCollection, rkey))
	if err != nil {
		return nil, err
	}
	if aturi.Authority().String() != did || aturi.Collection() != videoCollection || aturi.RecordKey().String() != rkey {
		return nil, errBadSession
	}
	if err := s.gateVideo(aturi.String(), did); err != nil {
		return nil, err
	}
	// VOD sessions are public-scope and unbound to an HTTP client.
	session, err := s.verifySession(ctx, sid, did, "")
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolveVideoBlob(ctx, aturi.String())
	if err != nil {
		return nil, err
	}
	meta, err := s.cachedMetafile(ctx, resolved.blobCID)
	if err != nil {
		return nil, err
	}
	out := &vod.ServableVideo{OwnerDID: did, SID: session.ID, BlobCID: resolved.blobCID, Tracks: make(map[string][]vod.ByteRange, len(meta.Tracks))}
	for id, track := range meta.Tracks {
		ranges := make([]vod.ByteRange, len(track.Segments))
		for i, seg := range track.Segments {
			ranges[i] = vod.ByteRange{Offset: meta.FlatHeaderSize + seg.Offset, Size: seg.Size}
		}
		out.Tracks[id] = ranges
	}
	return out, nil
}
