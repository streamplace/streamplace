package media

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/muxl"
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
	require.ErrorIs(t, mm.PublishRenditionsForPlayback(ctx, did, addendum, true), ErrAudioSourceMissing)
	require.Empty(t, sub.C)

	mm.rememberSourceAudio(ctx, did, src)
	require.NoError(t, mm.PublishRenditionsForPlayback(ctx, did, addendum, true))
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

func TestPublishRenditionsForPlaybackKeepsAvailableQualities(t *testing.T) {
	ctx := t.Context()
	src := sourceSegmentForTest(t)
	events, err := unwrapMuxlEvents(ctx, src)
	require.NoError(t, err)
	cat, tracks := catalogAndTracks(events)
	video, _, ok := pickVideoTrack(cat, onlyH264)
	require.True(t, ok)
	sourceVideo := tracks[strconv.FormatUint(uint64(video.TrackID()), 10)]
	require.NotEmpty(t, sourceVideo)
	cert, keyPEM := nodeSignerForTest(t)
	b := bus.NewBus()
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test"}, bus: b, nodeCert: cert, nodeKeyPEM: keyPEM}
	mm.nodeSignerOnce.Do(func() {})
	var addendum []byte
	for i, size := range [][2]int{{160, 120}, {80, 60}} {
		input := RenditionInput{MP4: renditionMP4(t, size[0], size[1]), Name: RenditionName(uint32(size[0]), uint32(size[1]))}
		pieces, timescale, err := mm.canonicalRenditionTrack(ctx, input, RenditionTrackID(i))
		require.NoError(t, err)
		delta, ok := retimeDelta(sourceVideo, video.Timescale(), pieces[0], timescale)
		require.True(t, ok)
		// Sign the second quality on a different timeline before combining the tracks.
		piece := shiftTfdt(pieces[0], delta+int64(i)*int64(timescale))
		signed, err := muxl.RunMuxlSignTranscode(ctx, muxl.TranscodeInput{
			Output: piece, Source: sourceVideo, CertPEM: cert, KeyPEM: keyPEM,
			Manifest: transcodeManifestFor(mm.cli.BroadcasterDID(), "transcoded video"),
		})
		require.NoError(t, err)
		addendum = append(addendum, signed...)
	}
	combined, err := unwrapMuxlEvents(ctx, addendum)
	require.NoError(t, err)
	_, combinedTracks := catalogAndTracks(combined)
	require.Len(t, combinedTracks, 2)
	const did = "did:test:streamer"
	require.NoError(t, mm.ValidateReplicatedRenditions(ctx, addendum, did))
	mm.rememberSourceAudio(ctx, did, src)
	available := b.SubscribeSegment(ctx, did, "120p")
	defer b.UnsubscribeSegment(ctx, did, "120p", available)
	unmatched := b.SubscribeSegment(ctx, did, "60p")
	defer b.UnsubscribeSegment(ctx, did, "60p", unmatched)
	require.NoError(t, mm.PublishRenditionsForPlayback(ctx, did, addendum, true))
	select {
	case segment := <-available.C:
		require.NotNil(t, segment.PacketizedData)
		require.NotEmpty(t, segment.PacketizedData.Video)
		require.NotEmpty(t, segment.PacketizedData.Audio)
	default:
		t.Fatal("available quality was withheld with the unmatched quality")
	}
	require.Empty(t, unmatched.C)
}
