package spxrpc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/cdn/bunny"
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
