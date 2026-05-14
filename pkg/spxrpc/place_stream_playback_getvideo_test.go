package spxrpc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

const fixtureURI = "at://did:plc:abc/place.stream.video/rkey1"

func TestMasterPlaylist(t *testing.T) {
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, nil, nil)
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
}

func TestMasterPlaylist_PropagatesTimeRange(t *testing.T) {
	start, end := int64(1_000_000_000), int64(2_000_000_000)
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, &start, &end)
	require.Contains(t, pl, "start=1000000000")
	require.Contains(t, pl, "end=2000000000")
}

func TestMediaPlaylist_Video(t *testing.T) {
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureURI, nil, nil)
	require.NoError(t, err)
	require.Contains(t, pl, "#EXT-X-PLAYLIST-TYPE:VOD")
	require.Contains(t, pl, "#EXT-X-INDEPENDENT-SEGMENTS")
	require.Contains(t, pl, `#EXT-X-MAP:URI=`)
	// .m4s suffix on the cid query value (cosmetic, for ffmpeg's
	// allowed_segment_extensions check). URL-encoded as `.m4s`.
	require.Contains(t, pl, "cid=bafyvideoinit.m4s")
	require.Contains(t, pl, "cid=bafyblob.m4s")
	require.Contains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
	require.Contains(t, pl, "#EXTINF:1.000000,")
	require.Contains(t, pl, "#EXT-X-ENDLIST")
}

func TestMediaPlaylist_UnknownTrack(t *testing.T) {
	_, err := mediaPlaylist(fixtureMetafile(), "99", fixtureURI, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TrackNotFound")
}

func TestMediaPlaylist_TimeRangeFilters(t *testing.T) {
	// Video track has two 1-second segments (timescale 6000, 6000
	// ticks each). Ask for [1s, 2s): should keep only segment 1.
	start := int64(1_000_000_000)
	end := int64(2_000_000_000)
	pl, err := mediaPlaylist(fixtureMetafile(), "1", fixtureURI, &start, &end)
	require.NoError(t, err)
	require.NotContains(t, pl, `#EXT-X-BYTERANGE:2000@100`)
	require.Contains(t, pl, `#EXT-X-BYTERANGE:1800@2100`)
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

// Make sure all .m3u8 outputs use LF line endings (no CRLF), match
// the HLS spec recommendation.
func TestPlaylistLineEndings(t *testing.T) {
	pl := masterPlaylist(fixtureMetafile(), fixtureURI, nil, nil)
	require.False(t, strings.Contains(pl, "\r"), "playlist should not contain carriage returns")
}
