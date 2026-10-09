package media

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

func TestWorkerConsumerFlushesFinalCanonicalGOP(t *testing.T) {
	ms, source := earlyWebRTCSource(t, false)
	mm := earlyWebRTCManager(t, ms)
	mm.cli.MaximumLiveBitrate = 1
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	feed, flush := mm.validateSegment(ctx)
	t.Cleanup(flush)
	require.NoError(t, feed(source))
	select {
	case <-mm.newSegmentSubs[0].queue:
		t.Fatal("one GOP must wait for AAC's final tail")
	default:
	}

	cancel()
	flush()
	select {
	case notification := <-mm.newSegmentSubs[0].queue:
		require.ElementsMatch(t, []string{"opus", "mp4a.40.2"}, audioCodecsOf(t, context.Background(), notification.Muxl))
		verified, err := muxl.RunMuxlVerify(context.Background(), bytes.NewReader(notification.Muxl))
		require.NoError(t, err)
		require.NotContains(t, verified, `"validation_state":"Invalid"`)
	case <-time.After(5 * time.Second):
		t.Fatal("consumer exit must flush canonical audio without waiting for the idle reaper")
	}
	mm.transcodersMu.Lock()
	remaining := len(mm.transcoders)
	mm.transcodersMu.Unlock()
	require.Zero(t, remaining, "the finished consumer must release its encoder")
	flush()
	select {
	case <-mm.newSegmentSubs[0].queue:
		t.Fatal("repeated consumer cleanup duplicated the final GOP")
	default:
	}
}

func TestWorkerConsumerFlushDoesNotCloseReplacement(t *testing.T) {
	ms, source := earlyWebRTCSource(t, false)
	mm := earlyWebRTCManager(t, ms)
	mm.cli.MaximumLiveBitrate = 1
	canonical := make(chan *NewSegmentNotification, 2)
	mm.newSegmentSubs = []*segmentSubscriber{{queue: canonical}}
	ctx := t.Context()
	firstFeed, firstFlush := mm.validateSegment(ctx)
	t.Cleanup(firstFlush)
	require.NoError(t, firstFeed(source))
	first := mm.transcoders[ms.Streamer()]
	require.NotNil(t, first)
	secondFeed, secondFlush := mm.validateSegment(ctx)
	t.Cleanup(secondFlush)
	require.NoError(t, secondFeed(source))
	second := mm.transcoders[ms.Streamer()]
	require.NotNil(t, second)
	require.NotSame(t, first, second)

	firstFlush()
	mm.transcodersMu.Lock()
	current := mm.transcoders[ms.Streamer()]
	mm.transcodersMu.Unlock()
	require.Same(t, second, current, "an old consumer must not remove its replacement")
	require.False(t, second.isClosed(), "an old consumer must not close its replacement")
	secondFlush()
	for range 2 {
		select {
		case <-canonical:
		case <-time.After(5 * time.Second):
			t.Fatal("both accepted sessions must finish their canonical GOP")
		}
	}
	select {
	case <-first.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the replaced encoder did not finish")
	}
}
