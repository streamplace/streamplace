package spxrpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"stream.place/streamplace/pkg/cdn"
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

	// A debugging escape hatch: nocdn=1 renders self-hosted segment URLs
	// even when a live CDN is configured, to tell a misbehaving zone from a
	// misbehaving node without a redeploy. Carried into the sub-playlists.
	live := s.live
	nocdn := c.QueryParam("nocdn") == "1"
	if nocdn {
		live = liveCDN{}
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
	rendition := c.QueryParam("rendition")

	// rendition=audio requests the primary audio track's media playlist
	// directly, skipping the master playlist so the player never loads video.
	if track == "" && rendition == "audio" {
		track = w.PrimaryAudioTrackID()
		if track == "" {
			return echo.NewHTTPError(http.StatusNotFound, "NoAudioTrack")
		}
	}

	var body string
	if track == "" {
		body = w.MasterPlaylist(func(tid string) string {
			return liveTrackPlaylistURL(did, tid, sid, nocdn)
		})
	} else {
		// The init always comes from the node: it can change mid-stream and
		// is one small fetch per session. Numbered segments go to the CDN
		// when one is configured.
		initURL := liveSegmentURL(did, track, "init", sid)
		segURI := func(seq uint64) string {
			return live.segmentURL(did, track, strconv.FormatUint(seq, 10), sid)
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
	return s.serveLiveSegment(c, c.QueryParam("streamer"), c.QueryParam("track"), c.QueryParam("seg"), c.QueryParam("sid"))
}

// liveSegmentPathRoute is the path-shaped form of getLiveSegment,
// /live/:streamer/:track/:seg, that live media playlists emit when a live
// CDN is configured. The CDN pulls it from this node, caches it under the
// segment's immutable Cache-Control, and (with a signing provider) checks
// the token bound to the path. No sid: the URL is identical for every
// viewer so the cache hits; the media playlist requests, which stay on the
// node, carry the session instead.
const liveSegmentPathRoute = "/live/:streamer/:track/:seg"

// HandleGetLiveSegmentPath serves liveSegmentPathRoute.
func (s *Server) HandleGetLiveSegmentPath(c echo.Context) error {
	return s.serveLiveSegment(c, c.Param("streamer"), c.Param("track"), c.Param("seg"), c.QueryParam("sid"))
}

// serveLiveSegment is the body of both segment routes.
func (s *Server) serveLiveSegment(c echo.Context, streamer, track, segParam, sid string) error {
	ctx := c.Request().Context()
	did, err := s.resolveStreamer(ctx, streamer)
	if err != nil {
		return err
	}
	if track == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "track is required")
	}
	// The cosmetic ".m4s" lets ffmpeg-based players fetch the URL; ignore it.
	seg := strings.TrimSuffix(segParam, ".m4s")
	if seg == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "seg is required")
	}

	w := s.mm.GetLiveWindow(did)
	if w == nil {
		return echo.NewHTTPError(http.StatusNotFound, "StreamNotLive")
	}

	// Segment fetches keep the playback session alive in the viewer count
	// (sid is threaded through every self-hosted segment URL by the
	// playlist handler; CDN-served segments carry none, and the playlist
	// requests that keep coming to the node are the heartbeat then).
	s.mm.TouchHLSSession(did, sid)

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
func liveTrackPlaylistURL(did, track, sid string, nocdn bool) string {
	q := url.Values{"streamer": {did}, "track": {track}}
	if sid != "" {
		q.Set("sid", sid)
	}
	if nocdn {
		q.Set("nocdn", "1")
	}
	return "/xrpc/place.stream.playback.getLivePlaylist?" + q.Encode()
}

// liveCDN is the live-segment CDN settings media playlists are generated
// against: a pull zone whose origin is this node. Zero-valued means
// self-hosted, and every segment URL points back at getLiveSegment.
type liveCDN struct {
	// URL is the CDN base URL. Empty means self-hosted.
	URL string
	// Signer turns a segment URL into what the player fetches. nil or
	// cdn.Static{} emits the URL unsigned.
	Signer cdn.Signer
	// TTL is the signed-URL lifetime bucket (see tokenExpiry).
	TTL time.Duration
	// now overrides the clock used for token expiry. nil = time.Now.
	now func() time.Time
}

// tokenExpiry is the expiry stamped into a signed live segment URL: the
// end of the *next* TTL bucket on the wall clock, so every playlist
// rendered within a bucket carries byte-identical URLs (a CDN caches by
// URL, and a per-render expiry would make every viewer's segment fetch a
// miss). A URL is therefore valid for between one and two TTLs, which is
// far longer than the segment stays in the live window.
func (l liveCDN) tokenExpiry() time.Time {
	now := time.Now
	if l.now != nil {
		now = l.now
	}
	ttl := l.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	t := now().Truncate(ttl)
	return t.Add(2 * ttl)
}

// segmentURL is the URL a media playlist emits for a numbered segment (or
// the init, which never goes through the CDN — see HandleGetLivePlaylist).
// Self-hosted, it is liveSegmentURL; behind a CDN it is the path-shaped
// route under the CDN host, signed when the provider signs.
func (l liveCDN) segmentURL(did, track, seg, sid string) string {
	if l.URL == "" || seg == "init" {
		return liveSegmentURL(did, track, seg, sid)
	}
	base, err := url.Parse(l.URL)
	if err != nil || base.Host == "" {
		// Not a parseable absolute URL: naive concatenation, unsigned.
		return strings.TrimRight(l.URL, "/") + liveSegmentPath(did, track, seg)
	}
	u := *base
	u.Path = strings.TrimRight(base.Path, "/") + liveSegmentPath(did, track, seg)
	u.RawPath = ""
	u.RawQuery = ""
	if l.Signer == nil {
		return u.String()
	}
	return l.Signer.SignURL(&u, l.tokenExpiry())
}

// liveSegmentPath is the path liveSegmentPathRoute matches for one segment.
func liveSegmentPath(did, track, seg string) string {
	return "/live/" + url.PathEscape(did) + "/" + url.PathEscape(track) + "/" + url.PathEscape(seg) + ".m4s"
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
