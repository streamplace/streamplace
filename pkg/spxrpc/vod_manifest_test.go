package spxrpc

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/vod"
)

func TestVideoManifest(t *testing.T) {
	meta := fixtureMetafile()
	meta.FlatHeaderSize = 123
	low := meta.Tracks["1"]
	low.Segments = append([]vod.MetafileSegment(nil), low.Segments...)
	for i := range low.Segments {
		low.Segments[i].Size /= 2
	}
	meta.Tracks["9"] = low
	opus := meta.Tracks["2"]
	opus.Codec = "opus"
	meta.Tracks["0"] = opus
	stem := opus
	stem.Group = 2
	meta.Tracks["3"] = stem
	playback := &videoPlayback{uri: fixtureURI, did: fixtureDID, session: playbackSession{SID: fixtureSID}, meta: meta}
	inits := map[string]string{"9": "aW5pdA=="}
	manifest := buildVideoManifest(playback, inits, vodCDN{}, "moqt://example.com:443?certhash=deadbeef")
	require.Equal(t, int64(2000), manifest.DurationMs)
	var ids []string
	for _, track := range manifest.Tracks {
		ids = append(ids, track.Id)
	}
	require.Equal(t, []string{"9", "1", "2", "0"}, ids)
	require.Equal(t, inits["9"], manifest.Tracks[0].Init)
	require.Equal(t, []int64{223, 2223}, manifest.Tracks[0].Offsets)
	require.Equal(t, []int64{1000, 900}, manifest.Tracks[0].Sizes)
	require.Equal(t, []int64{6000, 6000}, manifest.Tracks[0].Durations)
	require.Contains(t, manifest.Tracks[0].Url, "sid="+fixtureSID)
	require.Equal(t, "moq", manifest.Sources[0].Type)
	require.Equal(t, "https://example.com:443", *manifest.Sources[0].Url)
	require.Equal(t, "deadbeef", *manifest.Sources[0].Fingerprint)
	require.Equal(t, "vod/"+fixtureDID+"/rkey1/"+fixtureSID, *manifest.Sources[0].Broadcast)
	require.Equal(t, "http", manifest.Sources[1].Type)

	start, end := int64(1000), int64(1500)
	playback.start, playback.end = &start, &end
	clipped := buildVideoManifest(playback, inits, vodCDN{URL: "https://cdn.example", now: func() time.Time { return time.Unix(0, 0) }}, "moqt://example.com:443")
	require.Equal(t, int64(1000), clipped.DurationMs)
	require.Equal(t, int64(1), clipped.Tracks[0].First)
	require.Equal(t, []int64{2223}, clipped.Tracks[0].Offsets)
	require.Equal(t, "http", clipped.Sources[0].Type)
	require.Equal(t, "moq", clipped.Sources[1].Type)
	require.Nil(t, clipped.Sources[1].Fingerprint)
	require.True(t, strings.HasPrefix(clipped.Tracks[0].Url, "https://cdn.example/"))
	require.Equal(t, "http", manifestSources("", "", "", "", false)[0].Type)
}

func TestMasterPlaylistBandwidthOrder(t *testing.T) {
	meta := fixtureMetafile()
	low := meta.Tracks["1"]
	low.Segments = []vod.MetafileSegment{{Size: 10, DurationTicks: 6000}}
	meta.Tracks["9"] = low
	playlist := masterPlaylist(meta, fixtureURI, fixtureSID, nil, nil)
	require.Less(t, strings.Index(playlist, "track=9"), strings.Index(playlist, "track=1"))
}

func TestVODStarterBudget(t *testing.T) {
	key := make([]byte, vodStarterBudget-2)
	audio := []byte{1, 2}
	rest := []byte{3}
	starter := budgetStarter("1", "2", key, audio, rest)
	require.NotNil(t, starter.Video.Key)
	require.NotNil(t, starter.Audio.Data)
	require.Nil(t, starter.Video.Rest)
	decoded, err := base64.StdEncoding.DecodeString(*starter.Audio.Data)
	require.NoError(t, err)
	require.Equal(t, audio, decoded)
	starter = budgetStarter("1", "", make([]byte, vodStarterBudget+1), nil, rest)
	require.Nil(t, starter.Video.Key)
	require.Nil(t, starter.Audio)
	require.NotNil(t, starter.Video.Rest)
	starter = budgetStarter("1", "2", []byte{1}, make([]byte, vodStarterBudget), []byte{2})
	require.Nil(t, starter.Audio.Data)
	require.NotNil(t, starter.Video.Rest)
}

func TestVODStarterSkip(t *testing.T) {
	playback := &videoPlayback{}
	require.True(t, starterAllowed(playback, false))
	require.False(t, starterAllowed(playback, true))
	start := int64(1)
	playback.start = &start
	require.False(t, starterAllowed(playback, false))
	start = 0
	require.True(t, starterAllowed(playback, false))
}
