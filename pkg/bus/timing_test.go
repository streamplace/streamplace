package bus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSubscribeSegmentBufReplaysCachedSegments(t *testing.T) {
	b := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cached := &Seg{Filepath: "cached.mp4"}
	b.PublishSegment(ctx, "streamer", "source", cached)

	sub := b.SubscribeSegmentBuf(ctx, "streamer", "source", 1)
	select {
	case got := <-sub.C:
		require.Same(t, cached, got)
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive the cached segment")
	}
}

func TestPublishSegmentPreservesOrder(t *testing.T) {
	b := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := b.SubscribeSegment(ctx, "streamer", "source")
	defer b.UnsubscribeSegment(ctx, "streamer", "source", sub)
	segments := make([]*Seg, 64)
	for i := range segments {
		segments[i] = &Seg{}
		b.PublishSegment(ctx, "streamer", "source", segments[i])
	}

	for i, want := range segments {
		select {
		case got := <-sub.C:
			require.Same(t, want, got, "segment %d arrived out of order", i)
		case <-time.After(time.Second):
			t.Fatalf("segment %d was not delivered", i)
		}
	}
}

func TestSegmentTimingClonePreservesValuesWithoutSharingState(t *testing.T) {
	source := time.Unix(100, 0)
	timing := &SegmentTiming{
		SegmentID:          "segment-1",
		SourceStart:        source,
		IngestReceived:     source.Add(time.Second),
		Signed:             source.Add(2 * time.Second),
		MasteringCompleted: source.Add(3 * time.Second),
		Distributed:        source.Add(4 * time.Second),
		PacketizeStarted:   source.Add(5 * time.Second),
		PacketizeCompleted: source.Add(6 * time.Second),
		WebRTCQueued:       source.Add(7 * time.Second),
		WebRTCFirstSent:    source.Add(8 * time.Second),
	}

	clone := timing.Clone()
	require.Equal(t, timing, clone)
	require.NotSame(t, timing, clone)

	clone.Distributed = clone.Distributed.Add(time.Second)
	require.Equal(t, source.Add(4*time.Second), timing.Distributed)
}

func TestSegmentTimingSourceAge(t *testing.T) {
	now := time.Unix(110, 0)

	require.Equal(t, 10*time.Second, (&SegmentTiming{
		SourceStart: time.Unix(100, 0),
	}).SourceAge(now))
	require.Equal(t, time.Duration(0), (&SegmentTiming{
		SourceStart: time.Unix(120, 0),
	}).SourceAge(now))
	require.Equal(t, time.Duration(0), (*SegmentTiming)(nil).SourceAge(now))
	require.Equal(t, time.Duration(0), (&SegmentTiming{}).SourceAge(now))
}
