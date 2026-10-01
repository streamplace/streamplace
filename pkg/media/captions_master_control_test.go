package media

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func TestCaptionMasterWorkerControlAndReconnectIDs(t *testing.T) {
	ctx := context.Background()
	mm := NewOffline(&config.CLI{})
	var previousID string
	for range 2 {
		master := newCaptionMaster(ctx, "did:plc:streamer", &config.CLI{}, nil)
		master.setManifest(captionManifest("ingest"))
		master.clock(time.UnixMilli(200))
		master.mediaFinished = true
		path := filepath.Join(t.TempDir(), "ingest.sock")
		stop, err := master.servePush(path)
		require.NoError(t, err)
		unregister := mm.registerWorkerCaptionMaster(ctx, path, master.streamer)
		policy, live := mm.OriginCaptionPolicy(master.streamer)
		require.True(t, live)
		require.Equal(t, captions.CanonicalIngest, policy.Canonical)
		track := masterTrack(captions.SourceHuman)
		cue := captions.Cue{ID: "a", Start: master.arrival.Add(100 * time.Millisecond), End: master.arrival.Add(300 * time.Millisecond), Text: "supplied words"}
		require.NoError(t, mm.PushCanonicalCaptions(master.streamer, track, []captions.Cue{cue}))
		interim, err := master.text(ctx, muxl.TextRequest{EndMs: 250})
		require.NoError(t, err)
		require.Empty(t, interim.Tracks)
		cue.Final = true
		require.NoError(t, mm.PushCanonicalCaptions(master.streamer, track, []captions.Cue{cue}))
		final, err := master.text(ctx, muxl.TextRequest{StartMs: 250, EndMs: 1000})
		require.NoError(t, err)
		require.Len(t, final.Tracks, 1)
		require.Equal(t, "human", final.Tracks[0].Label)
		require.Len(t, final.Tracks[0].Cues, 1)
		got := final.Tracks[0].Cues[0]
		require.Equal(t, uint64(300), got.Start)
		require.Equal(t, uint64(500), got.End)
		require.Equal(t, "supplied words", got.Text)
		require.NotEqual(t, previousID, got.ID, "rapid reconnect cannot reuse a public canonical cue ID")
		previousID = got.ID
		master.setManifest(captionManifest("off"))
		policy, live = mm.OriginCaptionPolicy(master.streamer)
		require.True(t, live)
		require.Equal(t, captions.CanonicalOff, policy.Canonical)
		stop()
		unregister()
		_, live = mm.OriginCaptionPolicy(master.streamer)
		require.False(t, live)
	}
}
