package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The plugin's configuration request names the OBS mixer tracks; named tracks
// beyond the first become stems, each encoded as AAC and Opus beside the full
// mix, in track order and at most two of them. With nothing named the
// response carries no audio configuration at all, so the plugin keeps the
// single encoder from OBS's own settings.
func TestStemAudioConfigurations(t *testing.T) {
	decode := func(body string) clientConfigurationRequest {
		var req clientConfigurationRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return req
	}

	unnamed := decode(`{"authentication":"k","preferences":{"audio_tracks":[
		{"track_id":0,"name":""},{"track_id":1,"name":""},{"track_id":2,"name":""}]}}`)
	require.Nil(t, stemAudioConfigurations(unnamed.Preferences.AudioTracks), "unnamed tracks keep OBS's own encoder")
	require.Nil(t, stemAudioConfigurations(nil), "a request without preferences keeps OBS's own encoder")

	onlyFirstNamed := decode(`{"preferences":{"audio_tracks":[{"track_id":0,"name":"Mix"}]}}`)
	require.Nil(t, stemAudioConfigurations(onlyFirstNamed.Preferences.AudioTracks), "naming only the full mix makes no stems")

	// Three named stems arrive out of order: the two lowest mixer tracks win,
	// and the full mix (track 0) always comes first.
	named := decode(`{"preferences":{"audio_tracks":[
		{"track_id":0,"name":""},{"track_id":4,"name":"Game"},{"track_id":1,"name":"Music"},
		{"track_id":2,"name":""},{"track_id":3,"name":"Mic"},{"track_id":5,"name":"Alerts"}]}}`)
	got, err := json.Marshal(stemAudioConfigurations(named.Preferences.AudioTracks))
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"codec":"aac","track_id":0,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":0,"channels":2,"settings":{"bitrate":128}},
		{"codec":"aac","track_id":1,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":1,"channels":2,"settings":{"bitrate":128}},
		{"codec":"aac","track_id":3,"channels":2,"settings":{"bitrate":160}},
		{"codec":"opus","track_id":3,"channels":2,"settings":{"bitrate":128}}]`, string(got))
}
