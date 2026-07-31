package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/placestream"
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
	start, end := int64(1_000), int64(2_000) // milliseconds
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, fixtureSID, &start, &end)
	require.Contains(t, pl, "start=1000")
	require.Contains(t, pl, "end=2000")
}

func TestMediaPlaylist_Video(t *testing.T) {
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, "", nil, nil)
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
	// A clean single-session VOD has no discontinuities.
	require.NotContains(t, pl, "#EXT-X-DISCONTINUITY")
}

// reconnectVideoMetafile is a 3-segment video track whose middle segment is
// flagged as a discontinuity — the metafile shape produced by a recording that
// concatenates two ingest sessions (a reconnect), where session 2's decode
// time reset. Each segment is 1s (durationTicks == timescale).
func reconnectVideoMetafile() *vod.Metafile {
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
					{Offset: 2100, Size: 1800, DurationTicks: 6000, SampleCount: 60, Discontinuity: true},
					{Offset: 3900, Size: 1700, DurationTicks: 6000, SampleCount: 60},
				},
			},
		},
	}
}

func TestMediaPlaylist_Discontinuity(t *testing.T) {
	pl, err := mediaPlaylist(reconnectVideoMetafile(), "1", fixtureDID, fixtureSID, "", nil, nil)
	require.NoError(t, err)
	// Exactly one inline EXT-X-DISCONTINUITY (its own line — distinct from the
	// EXT-X-DISCONTINUITY-SEQUENCE header), and no sequence header for full play.
	require.Equal(t, 1, strings.Count(pl, "#EXT-X-DISCONTINUITY\n"))
	require.NotContains(t, pl, "#EXT-X-DISCONTINUITY-SEQUENCE")
	// It must sit after the first segment and immediately before the flagged one.
	discIdx := strings.Index(pl, "#EXT-X-DISCONTINUITY\n")
	firstSeg := strings.Index(pl, "#EXT-X-BYTERANGE:2000@100")
	flaggedSeg := strings.Index(pl, "#EXT-X-BYTERANGE:1800@2100")
	require.Greater(t, discIdx, firstSeg, "discontinuity must come after the first segment")
	require.Less(t, discIdx, flaggedSeg, "discontinuity must come right before the flagged segment")
}

func TestMediaPlaylist_DiscontinuityTrimmedToBoundary(t *testing.T) {
	start := int64(1000) // ms — segment index 1 (the boundary) starts at 1s
	pl, err := mediaPlaylist(reconnectVideoMetafile(), "1", fixtureDID, fixtureSID, "", &start, nil)
	require.NoError(t, err)
	// The boundary is now the first served segment: reflected as a sequence
	// bump, NOT an inline tag.
	require.Contains(t, pl, "#EXT-X-DISCONTINUITY-SEQUENCE:1")
	require.Equal(t, 0, strings.Count(pl, "#EXT-X-DISCONTINUITY\n"))
	require.NotContains(t, pl, "#EXT-X-BYTERANGE:2000@100", "pre-boundary segment should be trimmed")
	require.Contains(t, pl, "#EXT-X-BYTERANGE:1800@2100", "boundary segment should be served")
}

func TestMediaPlaylist_UnknownTrack(t *testing.T) {
	_, err := mediaPlaylist(fixtureMetafile(), "99", fixtureDID, fixtureSID, "", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TrackNotFound")
}

func TestMediaPlaylist_TimeRangeFilters(t *testing.T) {
	// Video track has two 1-second segments (timescale 6000, 6000
	// ticks each). Ask for [1000ms, 2000ms): should keep only segment 1.
	start := int64(1_000)
	end := int64(2_000)
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, "", &start, &end)
	require.NoError(t, err)
	require.NotContains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
}

func TestMediaPlaylist_EmptySIDOmitsParam(t *testing.T) {
	// Belt-and-suspenders for direct callers: passing an empty sid
	// shouldn't put a stray `sid=` in the URLs (URL-builder uses
	// url.Values.Set only when non-empty).
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, "", "", nil, nil)
	require.NoError(t, err)
	require.NotContains(t, pl, "sid=")
}

// TestMediaPlaylist_CDN exercises CDN-fronted output: segment + init
// URLs are absolute and point at the configured CDN under the baked-in
// blobs/ prefix, with did/sid as query params after the .mp4 path.
// The XRPC path must NOT appear.
func TestMediaPlaylist_CDN(t *testing.T) {
	const cdn = "https://cdn.example.com"
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, cdn, nil, nil)
	require.NoError(t, err)
	// Both the init segment and the content blob are served from the
	// CDN — no /xrpc/place.stream.playback.getVideoBlob anywhere.
	require.NotContains(t, pl, "/xrpc/place.stream.playback.getVideoBlob")
	require.Contains(t, pl, cdn+"/blobs/bafyvideoinit.mp4?")
	require.Contains(t, pl, cdn+"/blobs/bafyblob.mp4?")
	// Egress-accounting fields ride along on every blob URL.
	require.Contains(t, pl, "did=did%3Aplc%3Aabc")
	require.Contains(t, pl, "sid="+fixtureSID)
	// Byte-range + duration markers are unchanged — CDN doesn't affect
	// the segment layout, just the host of the blob.
	require.Contains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
}

func TestMediaPlaylist_CDNTrailingSlashNormalized(t *testing.T) {
	// `--vod-cdn-url=https://cdn.example.com/` shouldn't double-slash
	// between the base and the baked-in blobs/ prefix.
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureDID, fixtureSID, "https://cdn.example.com/", nil, nil)
	require.NoError(t, err)
	require.NotContains(t, pl, "//blobs/")
	require.Contains(t, pl, "https://cdn.example.com/blobs/bafyblob.mp4?")
}

func TestBlobURL_CDNWithPathPrefix(t *testing.T) {
	// A CDN URL with its own path is preserved verbatim; we still
	// append the baked-in blobs/<cid>.mp4 layout under it. Lets ops
	// front one CDN over multiple bucket sub-trees.
	got := blobURL("https://cdn.example.com/vods", "did:plc:abc", "bafyblob", "tid123")
	require.Equal(t, "https://cdn.example.com/vods/blobs/bafyblob.mp4?did=did%3Aplc%3Aabc&sid=tid123", got)
}

func TestBlobURL_SelfHosted(t *testing.T) {
	// Empty cdnURL keeps the existing XRPC URL shape. The .m4s suffix
	// stays at the end of the URL string for ffmpeg.
	got := blobURL("", "did:plc:abc", "bafyblob", "tid123")
	require.Equal(t,
		"/xrpc/place.stream.playback.getVideoBlob?did=did%3Aplc%3Aabc&sid=tid123&cid=bafyblob.m4s",
		got)
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

// TestResolveVideoBlob_SourceClip walks the playback-resolve path for
// a clip record: the clip's `Video` field points at a parent video
// whose sourceTracks lead to a real MediaTrack + blob CID. resolve
// should return the parent's blob CID and surface the clip's bounds.
func TestResolveVideoBlob_SourceClip(t *testing.T) {
	ctx := context.Background()
	m, err := indexdb.MakeDB(":memory:")
	require.NoError(t, err)
	s := &Server{model: m}

	const (
		owner     = "did:plc:owner"
		trackURI  = "at://did:plc:owner/place.stream.media.track/parent-track"
		parentURI = "at://did:plc:owner/place.stream.video/parent"
		clipURI   = "at://did:plc:owner/place.stream.video/clipof"
		parentCID = "bafyparentblob"
	)
	parseURI := func(s string) syntax.ATURI {
		u, err := syntax.ParseATURI(s)
		require.NoError(t, err)
		return u
	}

	require.NoError(t, m.UpsertMediaTrack(ctx, parseURI(trackURI), placestream.MediaTrack{
		LexiconTypeID: "place.stream.media.track",
		Track: placestream.MediaTrack_Track{
			MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{
				LexiconTypeID: "place.stream.media.defs#muxlTrack",
				Blob:          parentCID,
				TrackId:       "1",
				MediaType:     "video",
			},
		},
	}))

	parent := placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "parent",
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks: []comatproto.RepoStrongRef{
					{Uri: trackURI, Cid: "bafyrefcid"},
				},
			},
		},
	}
	require.NoError(t, m.UpsertVideo(ctx, parseURI(parentURI), parent))

	clip := placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "5s..10s of parent",
		Source: placestream.Video_Source{
			MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{
				LexiconTypeID: "place.stream.media.defs#sourceClip",
				Video:         parentURI,
				Start:         5_000,
				End:           10_000,
			},
		},
	}
	require.NoError(t, m.UpsertVideo(ctx, parseURI(clipURI), clip))

	t.Run("non-clip parent resolves with no bounds", func(t *testing.T) {
		got, err := s.resolveVideoBlob(ctx, parentURI)
		require.NoError(t, err)
		require.Equal(t, parentCID, got.blobCID)
		require.Equal(t, int64(0), got.clipStartMS)
		require.Nil(t, got.clipEndMS)
	})

	t.Run("clip resolves to parent blob + surfaces bounds", func(t *testing.T) {
		got, err := s.resolveVideoBlob(ctx, clipURI)
		require.NoError(t, err)
		require.Equal(t, parentCID, got.blobCID, "clip should serve the parent's content")
		require.Equal(t, int64(5_000), got.clipStartMS)
		require.NotNil(t, got.clipEndMS)
		require.Equal(t, int64(10_000), *got.clipEndMS)
	})

	t.Run("clip-of-clip is rejected", func(t *testing.T) {
		// A second clip pointing at the first clip should fail —
		// resolve only follows one hop and demands sourceTracks on
		// the parent.
		const nestedURI = "at://did:plc:owner/place.stream.video/nested"
		nested := placestream.Video{
			LexiconTypeID: "place.stream.video",
			Title:         "nested clip",
			Source: placestream.Video_Source{
				MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{
					LexiconTypeID: "place.stream.media.defs#sourceClip",
					Video:         clipURI,
					Start:         100,
					End:           200,
				},
			},
		}
		require.NoError(t, m.UpsertVideo(ctx, parseURI(nestedURI), nested))
		_, err := s.resolveVideoBlob(ctx, nestedURI)
		require.Error(t, err)
		he, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		require.Equal(t, http.StatusUnprocessableEntity, he.Code)
	})
}

func TestComposeClipBounds(t *testing.T) {
	intPtr := func(v int64) *int64 { return &v }
	deref := func(p *int64) (int64, bool) {
		if p == nil {
			return 0, false
		}
		return *p, true
	}

	for _, tc := range []struct {
		name             string
		clipStart        int64
		clipEnd          *int64 // nil = not a clip
		queryStart       *int64
		queryEnd         *int64
		wantStart        int64
		wantStartPresent bool
		wantEnd          int64
		wantEndPresent   bool
	}{
		{
			name: "non-clip, no query: full video",
		},
		{
			name:             "non-clip, query bounds: passthrough",
			queryStart:       intPtr(1_000),
			queryEnd:         intPtr(2_000),
			wantStart:        1_000,
			wantStartPresent: true,
			wantEnd:          2_000,
			wantEndPresent:   true,
		},
		{
			name:             "clip, no query: returns full clip in parent timeline",
			clipStart:        5_000,
			clipEnd:          intPtr(10_000),
			wantStart:        5_000,
			wantStartPresent: true,
			wantEnd:          10_000,
			wantEndPresent:   true,
		},
		{
			name:             "clip + query within bounds: translated by clipStart",
			clipStart:        5_000,
			clipEnd:          intPtr(10_000),
			queryStart:       intPtr(1_000),
			queryEnd:         intPtr(2_000),
			wantStart:        6_000,
			wantStartPresent: true,
			wantEnd:          7_000,
			wantEndPresent:   true,
		},
		{
			name:             "clip + query overshoots clip end: clamped",
			clipStart:        5_000,
			clipEnd:          intPtr(10_000),
			queryStart:       intPtr(1_000),
			queryEnd:         intPtr(60_000),
			wantStart:        6_000,
			wantStartPresent: true,
			wantEnd:          10_000,
			wantEndPresent:   true,
		},
		{
			name:             "clip + query at 0: translates to clipStart",
			clipStart:        5_000,
			clipEnd:          intPtr(10_000),
			queryStart:       intPtr(0),
			wantStart:        5_000,
			wantStartPresent: true,
			wantEnd:          10_000,
			wantEndPresent:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotStart, gotEnd := composeClipBounds(tc.clipStart, tc.clipEnd, tc.queryStart, tc.queryEnd)
			s, sp := deref(gotStart)
			e, ep := deref(gotEnd)
			require.Equal(t, tc.wantStartPresent, sp, "start presence")
			require.Equal(t, tc.wantEndPresent, ep, "end presence")
			if sp {
				require.Equal(t, tc.wantStart, s, "start value")
			}
			if ep {
				require.Equal(t, tc.wantEnd, e, "end value")
			}
		})
	}
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
