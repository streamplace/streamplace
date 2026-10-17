package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livehls"
)

// A node that receives a stream's rendition addendum (a syndicating node)
// packetizes each rendition with the source segment's Opus audio and
// publishes it on its bus under the rendition's name, so WebRTC viewers
// there can pick it.
func TestPublishRenditionsForPlayback(t *testing.T) {
	ctx := context.Background()
	src := sourceSegmentForTest(t)
	r160 := renditionMP4(t, 160, 120)
	cert, keyPEM := nodeSignerForTest(t)
	b := bus.NewBus()
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test"}, liveWindows: map[string]*livehls.Writer{}, bus: b}
	addendum, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "120p", MP4: r160}}, cert, keyPEM)
	require.NoError(t, err)

	const did = "did:test:streamer"
	sub := b.SubscribeSegment(ctx, did, "120p")
	defer b.UnsubscribeSegment(ctx, did, "120p", sub)

	// Without the source segment's audio there is nothing to pair with.
	mm.PublishRenditionsForPlayback(ctx, did, addendum, true)
	require.Empty(t, sub.C)

	mm.rememberSourceAudio(ctx, did, src)
	mm.PublishRenditionsForPlayback(ctx, did, addendum, true)
	select {
	case seg := <-sub.C:
		require.True(t, seg.Published)
		require.NotNil(t, seg.PacketizedData)
		require.NotEmpty(t, seg.PacketizedData.Video, "video samples")
		require.NotEmpty(t, seg.PacketizedData.Audio, "audio samples")
		require.Greater(t, seg.PacketizedData.Duration, time.Duration(0))
	case <-time.After(10 * time.Second):
		t.Fatal("no rendition segment published")
	}
}
