package viewlog

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
)

// loadStore returns an in-memory-ish FileStore + a writer pointed at
// it, sharing the same NodeDID across test cases so the file naming
// stays predictable.
func newAggTestWriter(t *testing.T, root string, nodeDID string, now time.Time) *Writer {
	t.Helper()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	w, err := NewWriter(Config{
		Store:      store,
		NodeDID:    nodeDID,
		FlushAfter: 1 * time.Hour,
		Salts:      NewSaltManager(newMemSaltStorage()),
		Now:        func() time.Time { return now },
	})
	require.NoError(t, err)
	return w
}

// runOne drives the writer's flush loop once, then closes.
func runOneAndClose(t *testing.T, ctx context.Context, w *Writer) {
	t.Helper()
	go w.Run(ctx)
	require.NoError(t, w.Close())
}

func TestAggregateWindowBasicView(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		sid1   = "tidsid1"
	)
	// One sid, one video, three segments — should count as one view.
	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: sid1, ManifestKind: ManifestKindMaster})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: sid1})
	w.Log(ctx, Event{Ts: now.Add(2 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: sid1})
	w.Log(ctx, Event{Ts: now.Add(3 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: sid1})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, videoA, res.VideoCounts[0].VideoURI)
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
}

func TestAggregateWindowDistinctSids(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		videoB = "at://did:plc:bob/place.stream.video/v2"
	)
	// 3 distinct sids watching videoA, 1 watching videoB.
	for i, sid := range []string{"sidA1", "sidA2", "sidA3"} {
		ts := now.Add(time.Duration(i) * time.Second)
		w.Log(ctx, Event{Ts: ts, Type: EventTypeManifestRequest, VideoURI: videoA, SID: sid})
		w.Log(ctx, Event{Ts: ts.Add(time.Millisecond), Type: EventTypeSegmentRequest, CID: "bafyA", SID: sid})
	}
	w.Log(ctx, Event{Ts: now.Add(10 * time.Second), Type: EventTypeManifestRequest, VideoURI: videoB, SID: "sidB1"})
	w.Log(ctx, Event{Ts: now.Add(11 * time.Second), Type: EventTypeSegmentRequest, CID: "bafyB", SID: "sidB1"})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
	})
	require.NoError(t, err)

	got := map[string]int64{}
	for _, vc := range res.VideoCounts {
		got[vc.VideoURI] = vc.Count
	}
	require.Equal(t, map[string]int64{videoA: 3, videoB: 1}, got)
}

func TestAggregateWindowThreshold(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const videoA = "at://did:plc:alice/place.stream.video/v1"
	// sidA fetched 1 segment, sidB fetched 3 — only sidB clears a
	// threshold of 2.
	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: "sidA"})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidA"})

	w.Log(ctx, Event{Ts: now.Add(2 * time.Second), Type: EventTypeManifestRequest, VideoURI: videoA, SID: "sidB"})
	w.Log(ctx, Event{Ts: now.Add(3 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidB"})
	w.Log(ctx, Event{Ts: now.Add(4 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidB"})
	w.Log(ctx, Event{Ts: now.Add(5 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidB"})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart:       now.Add(-time.Minute),
		WindowEnd:         now.Add(time.Minute),
		ThresholdSegments: 2,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.VideoCounts[0].Count, "only sids with ≥ threshold segments count")
}

func TestAggregateWindowSegmentsWithoutManifestDropped(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Segment events for a sid that never made a manifest request —
	// can't be attributed to a video, so they vanish.
	w.Log(ctx, Event{Ts: now, Type: EventTypeSegmentRequest, CID: "bafy", SID: "orphan"})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "orphan"})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.Empty(t, res.VideoCounts)
}

func TestAggregateWindowOutOfWindowEventsExcluded(t *testing.T) {
	root := t.TempDir()
	windowStart := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(5 * time.Minute)

	w := newAggTestWriter(t, root, "did:web:node1", windowStart)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const videoA = "at://did:plc:alice/place.stream.video/v1"
	// Segments before the window are ignored, segments in the window
	// count, segments after are ignored. The manifest event timestamp
	// is irrelevant — sid→video carries across the boundary.
	w.Log(ctx, Event{Ts: windowStart.Add(-time.Hour), Type: EventTypeManifestRequest, VideoURI: videoA, SID: "sidEarly"})
	w.Log(ctx, Event{Ts: windowStart.Add(-30 * time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidEarly"}) // dropped
	w.Log(ctx, Event{Ts: windowStart.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidEarly"})       // kept
	w.Log(ctx, Event{Ts: windowEnd.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafy", SID: "sidEarly"})         // dropped
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1, "the in-window segment still counts (sid→video carried from earlier manifest)")
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
}

func TestAggregateWindowCrossNodeTimeOrdering(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	// Two nodes, alphabetically reversed from time order: nodeZ writes
	// the earlier manifest, nodeA writes the later segment. Pure-key
	// lex sort would read the segment first and miss the attribution.
	wZ := newAggTestWriter(t, root, "did:web:nodeZ", now)
	wA := newAggTestWriter(t, root, "did:web:nodeA", now.Add(time.Minute))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wZ.Run(ctx)
	go wA.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		sid    = "wandering-sid"
	)
	wZ.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: sid})
	wA.Log(ctx, Event{Ts: now.Add(2 * time.Minute), Type: EventTypeSegmentRequest, CID: "bafy", SID: sid})
	require.NoError(t, wZ.Close())
	require.NoError(t, wA.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(10 * time.Minute),
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
}

func TestAggregateWindowDeterministicOrder(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	for i, video := range []string{
		"at://did:plc:c/place.stream.video/v1",
		"at://did:plc:a/place.stream.video/v1",
		"at://did:plc:b/place.stream.video/v1",
	} {
		sid := video // unique sid per video
		w.Log(ctx, Event{Ts: now.Add(time.Duration(i) * time.Second), Type: EventTypeManifestRequest, VideoURI: video, SID: sid})
		w.Log(ctx, Event{Ts: now.Add(time.Duration(i)*time.Second + time.Millisecond), Type: EventTypeSegmentRequest, CID: "bafy", SID: sid})
	}
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
	})
	require.NoError(t, err)
	got := make([]string, 0, len(res.VideoCounts))
	for _, vc := range res.VideoCounts {
		got = append(got, vc.VideoURI)
	}
	// Output is sorted by VideoURI so callers get a stable iteration.
	require.True(t, sort.StringsAreSorted(got), "VideoCounts should be sorted by URI for stable record rkeys, got %v", got)
}

func TestAggregateTaskKeyIsDeterministic(t *testing.T) {
	start := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	k1 := AggregateTaskKey(start, end)
	k2 := AggregateTaskKey(start, end)
	require.Equal(t, k1, k2, "same window must produce same key (dedups across nodes)")
	require.NotEqual(t, k1, AggregateTaskKey(start, end.Add(time.Minute)))
}

func TestViewCountRkeyShape(t *testing.T) {
	r := viewCountRkey("at://did:plc:alice/place.stream.video/v1", time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC))
	require.Len(t, r, 24)
	for _, c := range r {
		require.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"rkey character %q outside atproto's allowed set", c)
	}
	// Same inputs → same key (idempotent overwrites on rerun).
	require.Equal(t, r, viewCountRkey("at://did:plc:alice/place.stream.video/v1", time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)))
}
