package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

// The plugin's configuration request names the OBS mixer tracks; named tracks
// beyond the first become stems, each encoded like the full mix (AAC and Opus
// unless the streamer chose codecs), in track order and at most two of them.
// With nothing named and nothing chosen the response carries no audio
// configuration at all, so the plugin keeps the single encoder from OBS's own
// settings.
func TestAudioConfigurations(t *testing.T) {
	decode := func(body string) clientConfigurationRequest {
		var req clientConfigurationRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return req
	}

	unnamed := decode(`{"authentication":"k","preferences":{"audio_tracks":[
		{"track_id":0,"name":""},{"track_id":1,"name":""},{"track_id":2,"name":""}]}}`)
	require.Nil(t, audioConfigurations(unnamed.Preferences.AudioTracks, nil), "unnamed tracks keep OBS's own encoder")
	require.Nil(t, audioConfigurations(nil, nil), "a request without preferences keeps OBS's own encoder")

	onlyFirstNamed := decode(`{"preferences":{"audio_tracks":[{"track_id":0,"name":"Mix"}]}}`)
	require.Nil(t, audioConfigurations(onlyFirstNamed.Preferences.AudioTracks, nil), "naming only the full mix makes no stems")

	// Three named stems arrive out of order: the two lowest mixer tracks win,
	// and the full mix (track 0) always comes first.
	named := decode(`{"preferences":{"audio_tracks":[
		{"track_id":0,"name":""},{"track_id":4,"name":"Game"},{"track_id":1,"name":"Music"},
		{"track_id":2,"name":""},{"track_id":3,"name":"Mic"},{"track_id":5,"name":"Alerts"}]}}`)
	got, err := json.Marshal(audioConfigurations(named.Preferences.AudioTracks, nil))
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"codec":"aac","track_id":0,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":0,"channels":2,"settings":{"bitrate":128}},
		{"codec":"aac","track_id":1,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":1,"channels":2,"settings":{"bitrate":128}},
		{"codec":"aac","track_id":3,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":3,"channels":2,"settings":{"bitrate":128}}]`, string(got))

	// Chosen codecs replace the defaults, for the full mix alone too.
	opus := []*placestream.IngestAudioRendition{{Codec: "opus", Bitrate: 96}}
	got, err = json.Marshal(audioConfigurations(unnamed.Preferences.AudioTracks, opus))
	require.NoError(t, err)
	require.JSONEq(t, `[{"codec":"opus","track_id":0,"channels":2,"settings":{"bitrate":96}}]`, string(got))
	got, err = json.Marshal(audioConfigurations(named.Preferences.AudioTracks[:2], opus))
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"codec":"opus","track_id":0,"channels":2,"settings":{"bitrate":96}},
		{"codec":"opus","track_id":4,"channels":2,"settings":{"bitrate":96}}]`, string(got))

	// One encode per codec, so two stems stay within OBS's six encoders even
	// with a second record of a codec.
	three := []*placestream.IngestAudioRendition{{Codec: "aac", Bitrate: 160}, {Codec: "opus", Bitrate: 128}, {Codec: "aac", Bitrate: 96}}
	require.Len(t, audioConfigurations(named.Preferences.AudioTracks, three), 6)
}

// OBS encodes a rendition by dropping whole frames from its canvas, so only
// rates the canvas divides into evenly can be asked for.
func TestVideoConfigurationsFramerate(t *testing.T) {
	rate := func(n, d int64) placestream.IngestDefs_Framerate {
		return placestream.IngestDefs_Framerate{Numerator: n, Denominator: d}
	}
	at := func(r placestream.IngestDefs_Framerate) []*placestream.IngestVideoRendition {
		return []*placestream.IngestVideoRendition{{Codec: "h264", Width: 1920, Height: 1080, Framerate: r, Bitrate: 6000}}
	}
	for _, tc := range []struct {
		canvas, rendition placestream.IngestDefs_Framerate
		ok                bool
	}{
		{rate(60, 1), rate(60, 1), true},
		{rate(60, 1), rate(30, 1), true},
		{rate(60000, 1001), rate(30000, 1001), true},
		{rate(60, 1), rate(24, 1), false},
		{rate(30, 1), rate(60, 1), false},
		{rate(0, 0), rate(24, 1), true}, // OBS didn't say
	} {
		configs, problem := videoConfigurations(at(tc.rendition), []string{"h264"}, &tc.canvas)
		if tc.ok {
			require.Empty(t, problem, "%v from %v", tc.rendition, tc.canvas)
			require.Len(t, configs, 1)
		} else {
			require.Contains(t, problem, "can't be encoded", "%v from %v", tc.rendition, tc.canvas)
		}
	}
	configs, problem := videoConfigurations(at(rate(24, 1)), []string{"h264"}, nil)
	require.Empty(t, problem, "no canvas, no check")
	require.Len(t, configs, 1)
}

// Every rendition becomes a track, in record order, but only in codecs the
// plugin advertises: plugins before AV1 support get the H.264 renditions, or
// are told to update when there are none.
func TestVideoConfigurationsCodecs(t *testing.T) {
	renditions := []*placestream.IngestVideoRendition{
		{Codec: "av1", Width: 3840, Height: 2160, Framerate: placestream.IngestDefs_Framerate{Numerator: 120, Denominator: 1}},
		{Codec: "h264", Width: 1920, Height: 1080, Framerate: placestream.IngestDefs_Framerate{Numerator: 60, Denominator: 1}},
	}
	codecs := func(configs []videoConfiguration) (out []string) {
		for _, c := range configs {
			out = append(out, c.Codec)
		}
		return out
	}

	configs, problem := videoConfigurations(renditions, []string{"h264", "av1"}, nil)
	require.Empty(t, problem)
	require.Equal(t, []string{"av1", "h264"}, codecs(configs))

	configs, problem = videoConfigurations(renditions, []string{"h264"}, nil)
	require.Empty(t, problem)
	require.Equal(t, []string{"h264"}, codecs(configs))

	_, problem = videoConfigurations(renditions[:1], []string{"h264"}, nil)
	require.Contains(t, problem, "Update the plugin")
}
