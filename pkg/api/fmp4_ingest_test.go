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
}
