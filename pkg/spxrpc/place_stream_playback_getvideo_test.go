package spxrpc

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/vod"
)

// fixtureMetafile mirrors what muxl events produce for a typical h264 +
// aac VOD: one video track (id "1") and one audio track (id "2"), each
// with a per-track init segment and a handful of segments.
func fixtureMetafile() *vod.Metafile {
	return &vod.Metafile{
		BlobCID:  "bafyblob",
		BlobSize: 10_000,
		Tracks: map[string]vod.MetafileTrack{
			"1": {
				Type:      "video",
				Codec:     "avc1.64002a",
				Timescale: 6000,
				InitCID:   "bafyvideoinit",
				BlobCID:   "bafyblob",
				BlobSize:  10_000,
				Width:     1920,
				Height:    1080,
				Segments: []vod.MetafileSegment{
					{Offset: 100, Size: 2000, DurationTicks: 6000, SampleCount: 60},
					{Offset: 2100, Size: 1800, DurationTicks: 6000, SampleCount: 60},
				},
			},
			"2": {
				Type:       "audio",
				Codec:      "mp4a.40.2",
				Timescale:  48000,
				InitCID:    "bafyaudioinit",
				BlobCID:    "bafyblob",
				BlobSize:   10_000,
				Channels:   2,
				SampleRate: 48000,
				Segments: []vod.MetafileSegment{
					{Offset: 3900, Size: 600, DurationTicks: 48000, SampleCount: 50},
					{Offset: 4500, Size: 600, DurationTicks: 48000, SampleCount: 50},
				},
			},
		},
	}
}

const (
	fixtureURI = "at://did:plc:abc/place.stream.video/rkey1"
	fixtureDID = "did:plc:abc"
	// A real atproto TID — the same shape the handler hands out via
	// spid.TID() — so URL-embedding round-trips through ParseTID
	// without any encoding surprises.
	fixtureSID = "3jzfcijpj2z2a"
)

func TestMasterPlaylist(t *testing.T) {
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, fixtureSID, nil, nil)
	require.Contains(t, pl, "#EXTM3U")
	require.Contains(t, pl, "#EXT-X-VERSION:6")
	// Audio media line for the default (AAC) track.
	require.Contains(t, pl, `#EXT-X-MEDIA:TYPE=AUDIO`)
	require.Contains(t, pl, `GROUP-ID="audio"`)
	require.Contains(t, pl, `DEFAULT=YES`)
	// Video variant references audio group and includes both codecs.
	require.Contains(t, pl, `#EXT-X-STREAM-INF`)
	require.Contains(t, pl, `RESOLUTION=1920x1080`)
	require.Contains(t, pl, `CODECS="avc1.64002a,mp4a.40.2"`)
	require.Contains(t, pl, `AUDIO="audio"`)
	// Both per-track URIs go through getVideoPlaylist with the AT-URI
	// passed verbatim plus a track ID. URL-encoded `:` is `%3A` and
	// `/` is `%2F`.
	require.Contains(t, pl, "uri=at%3A%2F%2Fdid%3Aplc%3Aabc%2Fplace.stream.video%2Frkey1")
	require.Contains(t, pl, `track=1`)
	require.Contains(t, pl, `track=2`)
	// Session ID gets pushed into every per-track playlist URL so the
	// follow-up media-playlist + segment requests can be correlated.
	require.Contains(t, pl, "sid="+fixtureSID)
	require.Equal(t, 2, strings.Count(pl, "sid="+fixtureSID),
		"sid should appear once per track URL (audio + video variant)")
}

func TestMasterPlaylist_PropagatesTimeRange(t *testing.T) {
	start, end := int64(1_000_000_000), int64(2_000_000_000)
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, fixtureSID, &start, &end)
	require.Contains(t, pl, "start=1000000000")
	require.Contains(t, pl, "end=2000000000")
}

func TestMediaPlaylist_Video(t *testing.T) {
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, nil, nil)
	require.NoError(t, err)
	require.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:VOD")
	require.Contains(t, pl, "#EXT-X-INDEPENDENT-SEGMENTS")
	require.Contains(t, pl, `#EXT-X-MAP:URI=`)
	// .m4s suffix on the cid query value (cosmetic, for ffmpeg's
	// allowed_segment_extensions check). URL-encoded as `.m4s`.
	require.Contains(t, pl, "cid=bafyvideoinit.m4s")
	require.Contains(t, pl, "cid=bafyblob.m4s")
	// Owner DID carried for egress accounting.
	require.Contains(t, pl, "did=did%3Aplc%3Aabc")
	// cid must stay the last query param so the URL ends in `.m4s`;
	// `.m4s&` would mean another param got sorted after it. The sid
	// gets sorted between did and cid, so it can't break this.
	require.NotContains(t, pl, ".m4s&")
	// Session ID is on every segment + init URL.
	require.Contains(t, pl, "sid="+fixtureSID)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
	require.Contains(t, pl, "#EXTINF:1.000000,")
	require.Contains(t, pl, "#EXT-X-ENDLIST")
}

func TestMediaPlaylist_UnknownTrack(t *testing.T) {
	_, err := mediaPlaylist(fixtureMetafile(), "99", fixtureDID, fixtureSID, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TrackNotFound")
}

func TestMediaPlaylist_TimeRangeFilters(t *testing.T) {
	// Video track has two 1-second segments (timescale 6000, 6000
	// ticks each). Ask for [1s, 2s): should keep only segment 1.
	start := int64(1_000_000_000)
	end := int64(2_000_000_000)
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, &start, &end)
	require.NoError(t, err)
	require.NotContains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
}

func TestMediaPlaylist_EmptySIDOmitsParam(t *testing.T) {
	// Belt-and-suspenders for direct callers: passing an empty sid
	// shouldn't put a stray `sid=` in the URLs (URL-builder uses
	// url.Values.Set only when non-empty).
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, "", nil, nil)
	require.NoError(t, err)
	require.NotContains(t, pl, "sid=")
}

func TestSessionIDOrNew(t *testing.T) {
	t.Run("supplied is reused", func(t *testing.T) {
		// A well-formed TID like the player would have gotten from a
		// prior master-playlist response.
		valid := spid.TID()
		got, err := sessionIDOrNew(valid)
		require.NoError(t, err)
		require.Equal(t, valid, got)
	})
	t.Run("empty mints a fresh tid", func(t *testing.T) {
		got, err := sessionIDOrNew("")
		require.NoError(t, err)
		// The fresh sid is itself a valid TID — players that round-trip
		// it through ParseTID won't reject our output.
		_, err = syntax.ParseTID(got)
		require.NoError(t, err)
		// TIDs embed a timestamp + random clock id, so back-to-back
		// calls never collide.
		got2, err := sessionIDOrNew("")
		require.NoError(t, err)
		require.NotEqual(t, got, got2)
	})
	t.Run("invalid is rejected", func(t *testing.T) {
		for _, bad := range []string{
			"has space",      // whitespace
			"has/slash",      // URL-illegal char
			"has?question",   // ditto
			"tooshort",       // < 13 chars
			"toolongtoolong", // > 13 chars
			"1111111111111",  // 13 chars but '1' isn't in the TID alphabet
			"zabcdefghijkl",  // 13 chars but 'z' isn't a valid first char
		} {
			_, err := sessionIDOrNew(bad)
			require.Error(t, err, "expected %q to be rejected", bad)
		}
	})
}

func TestParseSingleRange(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		size      int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{name: "open ended", header: "bytes=100-", size: 1000, wantStart: 100, wantEnd: 999},
		{name: "closed", header: "bytes=100-200", size: 1000, wantStart: 100, wantEnd: 200},
		{name: "suffix", header: "bytes=-50", size: 1000, wantStart: 950, wantEnd: 999},
		{name: "closed clamped to end", header: "bytes=100-5000", size: 1000, wantStart: 100, wantEnd: 999},
		{name: "missing prefix", header: "100-200", size: 1000, wantErr: true},
		{name: "missing dash", header: "bytes=100", size: 1000, wantErr: true},
		{name: "multi-range rejected", header: "bytes=0-100,200-300", size: 1000, wantErr: true},
		{name: "start past end", header: "bytes=5000-", size: 1000, wantErr: true},
		{name: "end before start", header: "bytes=500-100", size: 1000, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parseSingleRange(tc.header, tc.size)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantStart, start)
			require.Equal(t, tc.wantEnd, end)
		})
	}
}

// TestHandleGetVideoPlaylist_RejectsBadSID exercises the handler-side
// validation. Validation runs before any DB / blob lookups, so the
// test doesn't need a populated model or playbackStore content — just
// the playbackStore presence check the handler does up front.
func TestHandleGetVideoPlaylist_RejectsBadSID(t *testing.T) {
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	s := &Server{playbackStore: store}

	for _, tc := range []struct {
		name string
		sid  string
	}{
		{"contains space", "has space"},
		{"contains slash", "has/slash"},
		{"too long", strings.Repeat("x", 65)},
		{"wrong tid alphabet", "1111111111111"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/xrpc/place.stream.playback.getVideoPlaylist?uri="+
					"at%3A%2F%2Fdid%3Aplc%3Aabc%2Fplace.stream.video%2Fr1"+
					"&sid="+url.QueryEscape(tc.sid), nil)
			c := echo.New().NewContext(req, httptest.NewRecorder())
			he := requireHTTPError(t, s.HandleGetVideoPlaylist(c))
			require.Equal(t, http.StatusBadRequest, he.Code)
		})
	}
}

// Make sure all .m3u8 outputs use LF line endings (no CRLF), match
// the HLS spec recommendation.
func TestPlaylistLineEndings(t *testing.T) {
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, fixtureSID, nil, nil)
	require.False(t, strings.Contains(pl, "\r"), "playlist should not contain carriage returns")
}
