package bus

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Segments reach a subscriber in the order they were published, and the
// recent ones are replayed to a subscriber that arrives late.
func TestPublishSegmentKeepsOrder(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		b.PublishSegment(ctx, "u", "source", &Seg{Filepath: fmt.Sprint("early", i)})
	}
	sub := b.SubscribeSegmentBuf(ctx, "u", "source", 2)
	defer b.UnsubscribeSegment(ctx, "u", "source", sub)
	const n = 500
	for i := 0; i < n; i++ {
		b.PublishSegment(ctx, "u", "source", &Seg{Filepath: fmt.Sprint(i)})
	}
	require.Equal(t, "early1", (<-sub.C).Filepath, "the last two are replayed")
	require.Equal(t, "early2", (<-sub.C).Filepath)
	for i := 0; i < n; i++ {
		require.Equal(t, fmt.Sprint(i), (<-sub.C).Filepath)
	}
}

// LatestSegment is the newest published segment, nil before the first one
// and after the session ends.
func TestLatestSegment(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	require.Nil(t, b.LatestSegment("u", "source"))
	for i := 0; i < bufSize+2; i++ {
		b.PublishSegment(ctx, "u", "source", &Seg{Filepath: fmt.Sprint(i)})
	}
	require.Equal(t, fmt.Sprint(bufSize+1), b.LatestSegment("u", "source").Filepath)
	require.Nil(t, b.LatestSegment("u", "720p"), "another rendition has its own cache")
	b.EndSession(ctx, "u", "source")
	require.Nil(t, b.LatestSegment("u", "source"))
}

// A subscriber that never reads does not hold the publisher.
func TestPublishSegmentDropsForStalledSubscriber(t *testing.T) {
	b := NewBus()
	ctx := context.Background()
	sub := b.SubscribeSegment(ctx, "u", "source")
	defer b.UnsubscribeSegment(ctx, "u", "source", sub)
	done := make(chan struct{})
	go func() {
		for i := 0; i < chanSize+50; i++ {
			b.PublishSegment(ctx, "u", "source", &Seg{})
		}
		close(done)
	}()
	<-done
	require.Len(t, sub.C, chanSize)
}

// The tracks a streamer sent alongside the source play by name, each with
// every codec it came in; the source's own name stays the source's.
func TestPacketizedRenditions(t *testing.T) {
	audio := []PacketizedSample{{Data: []byte("opus")}}
	p := &PacketizedSegment{Audio: audio, Video: []PacketizedVideo{
		{MimeType: "video/AV1", Name: "2160p"},
		{MimeType: "video/H264", Name: "2160p"},
		{MimeType: "video/AV1", Name: "720p"},
		{MimeType: "video/H264", Name: "1080p"},
		{MimeType: "video/H264", Name: "720p"},
	}}
	rs := p.Renditions()
	require.Len(t, rs, 2)
	require.Equal(t, []PacketizedVideo{{MimeType: "video/H264", Name: "1080p"}}, rs["1080p"].Video)
	require.Equal(t, []PacketizedVideo{{MimeType: "video/AV1", Name: "720p"}, {MimeType: "video/H264", Name: "720p"}}, rs["720p"].Video)
	require.Equal(t, audio, rs["720p"].Audio)
}
