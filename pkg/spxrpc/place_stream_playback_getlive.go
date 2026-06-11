package spxrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// --- stubs for the auto-generated wrappers in stubs.go ------------------
//
// Like getVideoBlob/getVideoPlaylist, the lexgen stubs hard-code status 200 +
// a fixed Content-Type, neither of which suits these endpoints (Range/206 on
// getLiveSegment, vnd.apple.mpegurl on getLivePlaylist). NewServer registers
// custom echo routes that override them; these exist only to satisfy the build.

func (s *Server) handlePlaceStreamPlaybackGetLivePlaylist(ctx context.Context, sid string, streamer string, track string) (io.Reader, error) {
	return nil, stubMisrouted("getLivePlaylist")
}

func (s *Server) handlePlaceStreamPlaybackGetLiveSegment(ctx context.Context, seg string, sid string, streamer string, track string) (io.Reader, error) {
	return nil, stubMisrouted("getLiveSegment")
}

// resolveStreamer turns a `streamer` query param — an alias, a DID
// (did:plc/did:web/did:key), or a Bluesky handle — into the streamer's DID,
// which is how the live window is keyed. Handles are resolved via the atproto
// sync cache; anything already a DID passes through unchanged.
func (s *Server) resolveStreamer(ctx context.Context, streamer string) (string, error) {
	if alias, ok := s.aliases[streamer]; ok {
		streamer = alias
	}
	if streamer == "" {
		return "", echo.NewHTTPError(http.StatusBadRequest, "streamer is required")
	}
	if strings.HasPrefix(streamer, "did:") {
		return streamer, nil
	}
	repo, err := s.ATSync.SyncBlueskyRepoCached(ctx, streamer)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, "could not resolve streamer handle: "+err.Error())
	}
	return repo.DID, nil
}

// --- getLivePlaylist ----------------------------------------------------

// HandleGetLivePlaylist serves a live HLS master playlist (track omitted) or a
// single-track media playlist out of the streamer's in-memory live window. The
// window is fed by ValidateMP4 for every segment that flows through this node,
// so a playlist exists only while the stream is live here. Open playback,
// gated only on an account ban (auth middleware can layer on later).
func (s *Server) HandleGetLivePlaylist(c echo.Context) error {
	ctx := c.Request().Context()
	did, err := s.resolveStreamer(ctx, c.QueryParam("streamer"))
	if err != nil {
		return err
	}

	if banned, err := s.accountBanned(did); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	} else if banned {
		return echo.NewHTTPError(http.StatusForbidden, "StreamUnavailable")
	}

	w := s.mm.GetLiveWindow(did)
	if w == nil {
		return echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
	}

	// Reuse the caller's sid when present so master/media/segment requests of
	// one playback session share an identifier; mint one otherwise.
	sid, err := sessionIDOrNew(c.QueryParam("sid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Sub-playlist + segment URLs carry the resolved DID, so follow-up requests
	// skip handle resolution and stay stable across a session.
	track := c.QueryParam("track")
	var body string
	if track == "" {
		body = w.MasterPlaylist(func(tid string) string {
			return liveTrackPlaylistURL(did, tid, sid)
		})
	} else {
		initURL := liveSegmentURL(did, track, "init", sid)
		segURI := func(seq uint64) string {
			return liveSegmentURL(did, track, strconv.FormatUint(seq, 10), sid)
		}
		body = w.MediaPlaylist(track, initURL, segURI)
		if body == "" {
			return echo.NewHTTPError(http.StatusNotFound, "TrackNotFound")
		}
		// A live player re-fetches the media playlist every target duration,
		// so this is the session's heartbeat into the viewer count. Master
		// requests don't count — one-shot fetchers (preview cards, health
		// checks) aren't viewers.
		s.mm.TouchHLSSession(did, sid)
	}

	h := c.Response().Header()
	h.Set("Content-Type", "application/vnd.apple.mpegurl")
	// The live media playlist changes as the window slides; never cache it.
	h.Set("Cache-Control", "no-cache")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Writer.Write([]byte(body))
	return err
}

// --- getLiveSegment -----------------------------------------------------

// HandleGetLiveSegment serves a track's init segment (seg=init) or a windowed
// canonical .m4s (seg=<media-sequence>) from the live window, with HTTP Range.
// The bytes are the verbatim signed segment, so provenance travels with
// playback.
func (s *Server) HandleGetLiveSegment(c echo.Context) error {
	ctx := c.Request().Context()
	did, err := s.resolveStreamer(ctx, c.QueryParam("streamer"))
	if err != nil {
		return err
	}
	track := c.QueryParam("track")
	if track == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "track is required")
	}
	// The cosmetic ".m4s" lets ffmpeg-based players fetch the URL; ignore it.
	seg := strings.TrimSuffix(c.QueryParam("seg"), ".m4s")
	if seg == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "seg is required")
	}

	w := s.mm.GetLiveWindow(did)
	if w == nil {
		return echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
	}

	// Segment fetches keep the playback session alive in the viewer count
	// (sid is threaded through every segment URL by the playlist handler).
	s.mm.TouchHLSSession(did, c.QueryParam("sid"))

	var data []byte
	isInit := seg == "init"
	if isInit {
		data = w.InitSegment(track)
	} else {
		seq, err := strconv.ParseUint(seg, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "seg must be 'init' or a media-sequence number")
		}
		data = w.SegmentData(track, seq)
	}
	if data == nil {
		return echo.NewHTTPError(http.StatusNotFound, "SegmentNotFound")
	}

	h := c.Response().Header()
	h.Set("Content-Type", "video/mp4")
	if isInit {
		// The init can change mid-stream (e.g. a resolution change mints a new
		// catalog), so don't let it be cached long.
		h.Set("Cache-Control", "no-cache")
	} else {
		// A numbered segment's bytes are fixed once minted (signed), so it's
		// safely immutable for as long as it stays in the window.
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	// http.ServeContent honors Range / sets Accept-Ranges + Content-Length and
	// keeps the Content-Type we set above.
	http.ServeContent(c.Response().Writer, c.Request(), "segment.m4s", time.Time{}, bytes.NewReader(data))
	return nil
}

// --- url builders -------------------------------------------------------

// liveTrackPlaylistURL is the URL to a single-track live media playlist served
// by this same handler. did is the resolved streamer DID; sid is propagated so
// a player's playlist + segment requests share an identifier.
func liveTrackPlaylistURL(did, track, sid string) string {
	q := url.Values{"streamer": {did}, "track": {track}}
	if sid != "" {
		q.Set("sid", sid)
	}
	return "/xrpc/place.stream.playback.getLivePlaylist?" + q.Encode()
}

// liveSegmentURL builds a getLiveSegment URL. seg is "init" or a media
// sequence number; the trailing ".m4s" is appended last (so seg is the final
// query token) to satisfy ffmpeg's segment-extension allowlist — the handler
// strips it back off.
func liveSegmentURL(did, track, seg, sid string) string {
	q := url.Values{"streamer": {did}, "track": {track}}
	if sid != "" {
		q.Set("sid", sid)
	}
	return "/xrpc/place.stream.playback.getLiveSegment?" + q.Encode() +
		"&seg=" + url.QueryEscape(seg) + ".m4s"
}
