package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/cdn/bunny"
	"stream.place/streamplace/pkg/psession"
)

// TestLiveSegmentURL pins the ffmpeg-compatibility contract: the segment URL
// must END in ".m4s" (ffmpeg's HLS demuxer checks the URL extension against
// its allowlist before fetching), and the handler must be able to recover the
// did/track/seg from it.
func TestLiveSegmentURL(t *testing.T) {
	u := liveSegmentURL("did:plc:abc123", "1", "42", "3kabc")
	require.True(t, strings.HasSuffix(u, ".m4s"), "segment URL must end in .m4s, got %q", u)
	require.True(t, strings.HasPrefix(u, "/xrpc/place.stream.playback.getLiveSegment?"))
	require.Contains(t, u, "track=1")
	require.Contains(t, u, "sid=3kabc")
	require.Contains(t, u, "seg=42.m4s")
	// The resolved streamer DID is percent-encoded under the streamer param.
	require.Contains(t, u, "streamer=did%3Aplc%3Aabc123")

	// init segment URL (no sid).
	ui := liveSegmentURL("did:plc:abc123", "2", "init", "")
	require.True(t, strings.HasSuffix(ui, "seg=init.m4s"), "init URL must end in seg=init.m4s, got %q", ui)
	require.NotContains(t, ui, "sid=")
}

// TestLiveTrackPlaylistURL confirms the master playlist's per-track sub-URLs
// carry did + track + sid back to getLivePlaylist.
func TestLiveTrackPlaylistURL(t *testing.T) {
	u := liveTrackPlaylistURL("did:plc:xyz", "1", "3ksid", false)
	require.True(t, strings.HasPrefix(u, "/xrpc/place.stream.playback.getLivePlaylist?"))
	require.Contains(t, u, "track=1")
	require.Contains(t, u, "sid=3ksid")
	require.Contains(t, u, "streamer=did%3Aplc%3Axyz")
	require.NotContains(t, u, "nocdn")
	// The escape hatch follows the master into its sub-playlists.
	require.Contains(t, liveTrackPlaylistURL("did:plc:xyz", "1", "3ksid", true), "nocdn=1")
}

// TestLiveCDNSegmentURL pins the CDN contract: numbered segments move to the
// path-shaped route under the CDN host with no per-viewer query string, the
// init stays on the node, and a zero liveCDN is exactly the self-hosted form.
func TestLiveCDNSegmentURL(t *testing.T) {
	var self liveCDN
	require.Equal(t, liveSegmentURL("did:plc:abc", "1", "42", "sid1"), self.segmentURL("did:plc:abc", "1", "42", "sid1"))

	l := liveCDN{URL: "https://live.b-cdn.net", TTL: time.Minute}
	u := l.segmentURL("did:plc:abc", "1", "42", "sid1")
	require.Equal(t, "https://live.b-cdn.net/live/did:plc:abc/1/42.m4s", u)
	require.NotContains(t, u, "sid", "CDN URLs carry no session so every viewer shares the cache entry")
	require.True(t, strings.HasSuffix(u, ".m4s"), "ffmpeg's extension allowlist")
	// The init is one small per-session fetch and can change mid-stream:
	// it stays on the node, session and all.
	require.Equal(t, liveSegmentURL("did:plc:abc", "1", "init", "sid1"), l.segmentURL("did:plc:abc", "1", "init", "sid1"))
	// A base URL with a path prefix keeps it.
	require.Equal(t, "https://cdn.example.com/edge/live/did:plc:abc/1/42.m4s",
		liveCDN{URL: "https://cdn.example.com/edge/"}.segmentURL("did:plc:abc", "1", "42", ""))
	// The route pattern and the emitted path agree.
	require.Equal(t, "/live/:streamer/:track/:seg", liveSegmentPathRoute)
	pu, err := url.Parse(u)
	require.NoError(t, err)
	require.Equal(t, "/live/did:plc:abc/1/42.m4s", pu.Path)
}

// TestLiveCDNSignedURLStableWithinBucket: signing puts `expires` in the
// query, and a CDN caches by URL, so the expiry must not change between
// two playlist renders a few seconds apart or no viewer ever shares a
// cached segment. Two renders inside one TTL bucket must be byte-identical;
// the next bucket rolls the token over; every token outlives its bucket.
func TestLiveCDNSignedURLStableWithinBucket(t *testing.T) {
	base := time.Date(2026, 9, 10, 12, 0, 7, 0, time.UTC)
	clock := base
	l := liveCDN{URL: "https://live.b-cdn.net", Signer: bunny.Signer{Key: "k"}, TTL: time.Minute, now: func() time.Time { return clock }}
	u1 := l.segmentURL("did:plc:abc", "1", "42", "")
	clock = base.Add(40 * time.Second)
	u2 := l.segmentURL("did:plc:abc", "1", "42", "")
	require.Equal(t, u1, u2, "same bucket, same URL")
	require.Contains(t, u1, "token=")
	require.Contains(t, u1, "expires=")
	require.NotContains(t, u1, "sid=")
	clock = base.Add(61 * time.Second)
	u3 := l.segmentURL("did:plc:abc", "1", "42", "")
	require.NotEqual(t, u1, u3, "next bucket, fresh token")
	// expiry = end of the bucket after the current one.
	exp := l.tokenExpiry()
	require.Equal(t, time.Date(2026, 9, 10, 12, 3, 0, 0, time.UTC), exp)
	require.True(t, exp.Sub(clock) > time.Minute && exp.Sub(clock) <= 2*time.Minute)
}

// TestLiveSegmentPathRouteParams: what liveCDN.segmentURL emits, the path
// route must take apart again — DID with its colons, track, and the seg
// with its cosmetic .m4s — so the CDN's origin fetch lands on the right
// segment.
func TestLiveSegmentPathRouteParams(t *testing.T) {
	e := echo.New()
	var got []string
	e.GET(liveSegmentPathRoute, func(c echo.Context) error {
		got = []string{c.Param("streamer"), c.Param("track"), c.Param("seg")}
		return c.NoContent(http.StatusOK)
	})
	u := liveCDN{URL: "https://live.b-cdn.net"}.segmentURL("did:plc:abc", "1", "42", "")
	pu, err := url.Parse(u)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, pu.Path, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []string{"did:plc:abc", "1", "42.m4s"}, got)
	require.Equal(t, "42", strings.TrimSuffix(got[2], ".m4s"))
}

// TestResolveSession pins the session contract on playback requests: a
// first request gets a fresh public session (and a media playlist is
// redirected to carry it), a valid one is kept, one past half its life or
// expired is renewed under the same id, and a forged one is refused.
func TestResolveSession(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	s := &Server{}
	s.sessionKeyCache.Store(&key)
	ctx := context.Background()
	const streamer = "did:plc:abc"

	fresh, err := s.resolveSession(ctx, "", streamer, true)
	require.NoError(t, err)
	require.True(t, fresh.Redirect, "a media playlist without a session is redirected to one")
	require.Equal(t, psession.ScopePublic, fresh.Scope)
	require.Equal(t, fresh.ID, psession.ID(fresh.SID))

	master, err := s.resolveSession(ctx, "", streamer, false)
	require.NoError(t, err)
	require.False(t, master.Redirect, "a master playlist embeds the session it mints")

	kept, err := s.resolveSession(ctx, fresh.SID, streamer, true)
	require.NoError(t, err)
	require.False(t, kept.Redirect)
	require.Equal(t, fresh.SID, kept.SID, "a young session is emitted as is")

	_, err = s.resolveSession(ctx, fresh.SID, "did:plc:other", true)
	require.ErrorIs(t, err, errBadSession, "bound to the stream it was minted for")
	_, err = s.resolveSession(ctx, "3kabc", streamer, true)
	require.ErrorIs(t, err, errBadSession, "a bare id is not a session")

	// Past half-life: renewed under the same id, and a media playlist is
	// redirected to the renewal so the player's follow-ups carry it (a
	// master embeds it).
	aging, _ := psession.Mint(key, streamer, psession.ScopePublic, publicSessionTTL/3, time.Now())
	_, agingSID := psession.Renew(key, aging, streamer, publicSessionTTL/3, time.Now())
	half, err := s.resolveSession(ctx, agingSID, streamer, true)
	require.NoError(t, err)
	require.True(t, half.Redirect)
	require.Equal(t, aging.ID, half.ID)
	require.NotEqual(t, agingSID, half.SID)
	halfMaster, err := s.resolveSession(ctx, agingSID, streamer, false)
	require.NoError(t, err)
	require.False(t, halfMaster.Redirect)
	require.NotEqual(t, agingSID, halfMaster.SID, "renewed all the same")

	// Expired: renewed under the same id; a media playlist is redirected to
	// the renewal so the player's follow-ups carry it.
	old, _ := psession.Mint(key, streamer, psession.ScopePublic, -time.Hour, time.Now())
	_, oldSID := psession.Renew(key, old, streamer, -time.Hour, time.Now())
	renewed, err := s.resolveSession(ctx, oldSID, streamer, true)
	require.NoError(t, err)
	require.True(t, renewed.Redirect)
	require.Equal(t, old.ID, renewed.ID)
	require.NotEqual(t, oldSID, renewed.SID)

	// A blob request is credited to its session only when the session
	// verifies for the owner; a sid copied across streams credits nothing.
	require.Equal(t, fresh.ID, s.accountedSession(ctx, fresh.SID, streamer))
	require.Equal(t, "", s.accountedSession(ctx, fresh.SID, "did:plc:other"))
	require.Equal(t, "", s.accountedSession(ctx, "3kabc", streamer))
	require.Equal(t, old.ID, s.accountedSession(ctx, oldSID, streamer), "expired but genuine still names its session")

	// Segments never mint: a URL out of a playlist carries a session or is
	// refused.
	sess, err := s.verifySession(ctx, renewed.SID, streamer)
	require.NoError(t, err)
	require.Equal(t, old.ID, sess.ID)
	_, err = s.verifySession(ctx, "", streamer)
	require.Error(t, err)
	_, err = s.verifySession(ctx, oldSID, streamer)
	require.Error(t, err, "expired")

	// The owner session: minted by getPlaybackSession, it is what opens a
	// pre-live stream; a public one does not (the handler checks Scope).
	_, ownerSID := psession.Mint(key, streamer, psession.ScopeOwner, ownerSessionTTL, time.Now())
	owner, err := s.resolveSession(ctx, ownerSID, streamer, true)
	require.NoError(t, err)
	require.Equal(t, psession.ScopeOwner, owner.Scope)
}

// TestRedirectWithSession: the redirect keeps the request's URL and adds the
// session, and is never cached.
func TestRedirectWithSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/xrpc/place.stream.playback.getLivePlaylist?streamer=did%3Aplc%3Aabc&track=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	require.NoError(t, redirectWithSession(c, "1.3kabc.p.1.sig"))
	require.Equal(t, http.StatusFound, rec.Code)
	loc, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/xrpc/place.stream.playback.getLivePlaylist", loc.Path)
	require.Equal(t, "did:plc:abc", loc.Query().Get("streamer"))
	require.Equal(t, "1", loc.Query().Get("track"))
	require.Equal(t, "1.3kabc.p.1.sig", loc.Query().Get("sid"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}
