package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Missing files are 404s; app routes (including handles that end in a TLD)
// still get the app shell.
func TestLooksLikeAsset(t *testing.T) {
	for _, p := range []string{"blobs/bafkr4ihdrzk.mp4", "blobs/bafkr4ihdrzk.json", "live/did:plc:x/1/42.m4s", "_expo/static/js/web/old-hash.js", "x.M3U8"} {
		require.True(t, looksLikeAsset(p), p)
	}
	for _, p := range []string{"", "eli.bsky.social", "live.example.eu", "live.example.eu/video/3mvmy2nvqbmqv", "settings", "video", "docs/getting-started/"} {
		require.False(t, looksLikeAsset(p), p)
	}
}
