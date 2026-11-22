package spxrpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/psession"
)

// Playback sessions (pkg/psession) on the playback handlers. Every playlist
// and segment URL a handler emits carries the viewer's signed session as sid;
// the handlers verify it against the stream it was minted for, renew it while
// the viewer keeps watching, and hand a viewer without one a fresh one.

const (
	// publicSessionTTL is how long a viewer's session lasts without being
	// renewed. A live player refreshes its playlist every few seconds and
	// every playlist rendered renews a session past half its life, so an
	// attentive viewer's session never lapses; a URL copied out of a
	// playlist stops working after this.
	publicSessionTTL = 24 * time.Hour
	// ownerSessionTTL is the streamer's own session (pre-live preview). It is
	// never renewed: the app mints a new one before this runs out.
	ownerSessionTTL = time.Hour
)

// sessionKey is the station's signing key, read from statedb once: session
// checks happen on every playlist and segment request, and the key never
// changes once it exists.
func (s *Server) sessionKey(ctx context.Context) ([]byte, error) {
	if key := s.sessionKeyCache.Load(); key != nil {
		return *key, nil
	}
	if s.statefulDB == nil {
		return nil, fmt.Errorf("no statedb")
	}
	key, err := s.statefulDB.EnsurePlaybackSessionKey(ctx)
	if err != nil {
		return nil, err
	}
	s.sessionKeyCache.Store(&key)
	return key, nil
}

// sessionClient is what an owner session is bound to: the client's IPv4
// address, or its IPv6 /64 so privacy-address rotation doesn't break it. The
// address is the edge proxy's forwarded client when the request carries this
// node's proxy secret, so a redirect to the node keeps matching.
func (s *Server) sessionClient(c echo.Context) string {
	raw := s.cli.ClientIP(c.Request().Header, c.Request().RemoteAddr)
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String()
}

// playbackSession is what a playback request resolved to: the session to
// count by and the sid to emit in every URL of the response.
type playbackSession struct {
	psession.Session
	// SID is the value to put in emitted URLs: the caller's, or a renewal
	// of it (same ID, fresh expiry), or a fresh session.
	SID string
	// Redirect is set when the caller sent no usable session for a request
	// whose URL the player will keep re-fetching (a media playlist): the
	// handler redirects to the same URL carrying SID, so the player's
	// follow-ups share one session instead of minting one per poll.
	Redirect bool
}

// xrpcError is an XRPC error response; echo writes a non-error Message as is.
func xrpcError(code int, name, message string) *echo.HTTPError {
	return echo.NewHTTPError(code, map[string]string{"error": name, "message": message})
}

var errBadSession = xrpcError(http.StatusBadRequest, "InvalidSession", "playback session is invalid or expired")

// errPlaybackElsewhere refuses a pre-live playback whose session a newer one
// of the streamer's has replaced.
var errPlaybackElsewhere = xrpcError(http.StatusConflict, "PlaybackStartedElsewhere", "pre-live playback started elsewhere")

// resolveSession verifies the sid a playback request from client carries
// against streamer, or mints one. A malformed or forged sid is refused; an
// expired public one is renewed in place, so a viewer who paused for a day
// resumes with their own session id. An owner session is never renewed. When
// needsRedirect is set and the caller had no valid session, the result asks
// for a redirect.
func (s *Server) resolveSession(ctx context.Context, sid, streamer, client string, needsRedirect bool) (playbackSession, error) {
	if sid != "" && !psession.WellFormed(sid) {
		return playbackSession{}, errBadSession
	}
	key, err := s.sessionKey(ctx)
	if err != nil {
		return playbackSession{}, err
	}
	now := time.Now()
	if sid == "" {
		sess, value := psession.Mint(key, streamer, client, psession.ScopePublic, publicSessionTTL, now)
		return playbackSession{Session: sess, SID: value, Redirect: needsRedirect}, nil
	}
	sess, err := psession.Parse(key, sid, streamer, client, now)
	switch {
	case err == nil:
		out := playbackSession{Session: sess, SID: sid}
		if sess.Scope == psession.ScopePublic && sess.Expires.Sub(now) < publicSessionTTL/2 {
			// Past half-life: renew, and send a media playlist to the
			// renewed URL, or the player would keep polling the old one and
			// every poll would renew (and re-sign every segment URL) again.
			out.Session, out.SID = psession.Renew(key, sess, streamer, client, publicSessionTTL, now)
			out.Redirect = needsRedirect
		}
		return out, nil
	case errors.Is(err, psession.ErrExpired) && sess.Scope == psession.ScopePublic:
		sess, value := psession.Renew(key, sess, streamer, client, publicSessionTTL, now)
		return playbackSession{Session: sess, SID: value, Redirect: needsRedirect}, nil
	default:
		return playbackSession{}, errBadSession
	}
}

// verifySession is resolveSession for a segment request: the URL came out
// of a playlist this node rendered, so it must carry a valid session or it
// is refused, nothing is minted.
func (s *Server) verifySession(ctx context.Context, sid, streamer, client string) (psession.Session, error) {
	key, err := s.sessionKey(ctx)
	if err != nil {
		return psession.Session{}, err
	}
	sess, err := psession.Parse(key, sid, streamer, client, time.Now())
	if err != nil {
		return psession.Session{}, echo.NewHTTPError(http.StatusForbidden, "InvalidSession")
	}
	return sess, nil
}

// accountedSession is the session id a request is counted under: the
// session's, when it verifies for streamer; nothing otherwise. A copied or
// forged sid must not credit views and bytes to someone else's session.
func (s *Server) accountedSession(ctx context.Context, sid, streamer, client string) string {
	if sid == "" {
		return ""
	}
	key, err := s.sessionKey(ctx)
	if err != nil {
		return ""
	}
	sess, err := psession.Parse(key, sid, streamer, client, time.Now())
	if err != nil && !errors.Is(err, psession.ErrExpired) {
		return ""
	}
	return sess.ID
}

// redirectWithSession answers a request that arrived without a usable
// session: the same URL, with the session attached.
func redirectWithSession(c echo.Context, sid string) error {
	u := *c.Request().URL
	q := u.Query()
	q.Set("sid", sid)
	u.RawQuery = q.Encode()
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Redirect(http.StatusFound, u.RequestURI())
}

// withSID adds the session to a set of URL query values.
func withSID(q url.Values, sid string) url.Values {
	if sid != "" {
		q.Set("sid", sid)
	}
	return q
}

// handlePlaceStreamPlaybackGetPlaybackSession mints the caller's own
// session for their stream: it opens their unpublished (pre-live) stream to
// their own player, which the anonymous session a playlist hands out does
// not. Bound to the caller's DID and client address, so it opens nobody
// else's stream and is useless copied to anyone else.
func (s *Server) handlePlaceStreamPlaybackGetPlaybackSession(ctx context.Context) (*placestream.PlaybackGetPlaybackSession_Output, error) {
	session := GetCaller(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	c, ok := ctx.Value(echoContextKey).(echo.Context)
	if !ok {
		return nil, fmt.Errorf("no request context")
	}
	key, err := s.sessionKey(ctx)
	if err != nil {
		return nil, err
	}
	sess, sid := psession.Mint(key, session.DID, s.sessionClient(c), psession.ScopeOwner, ownerSessionTTL, time.Now())
	return &placestream.PlaybackGetPlaybackSession_Output{Sid: sid, ExpiresAt: sess.Expires.Format(time.RFC3339)}, nil
}
