package spxrpc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	u := liveTrackPlaylistURL("did:plc:xyz", "1", "3ksid")
	require.True(t, strings.HasPrefix(u, "/xrpc/place.stream.playback.getLivePlaylist?"))
	require.Contains(t, u, "track=1")
	require.Contains(t, u, "sid=3ksid")
	require.Contains(t, u, "streamer=did%3Aplc%3Axyz")
}
