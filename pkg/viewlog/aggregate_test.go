package viewlog

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/vod"
)

// fixtureTrackRefs returns the strongRefs that pair with the fixture
// metafile, keyed by in-container trackId. Same shape the real
// resolver in cmd/streamplace.go produces from MediaTrack rows.
func fixtureTrackRefs() map[string]*comatproto.RepoStrongRef {
	return map[string]*comatproto.RepoStrongRef{
		"1": {
			LexiconTypeID: "com.atproto.repo.strongRef",
			Uri:           "at://did:plc:alice/place.stream.media.track/track-video-1",
			Cid:           "bafytrack1",
		},
		"2": {
			LexiconTypeID: "com.atproto.repo.strongRef",
			Uri:           "at://did:plc:alice/place.stream.media.track/track-audio-2",
			Cid:           "bafytrack2",
		},
	}
}

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
	videoURI := "at://did:plc:alice/place.stream.video/3jw5xvr5gck2a"
	windowStart := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	r, err := viewCountRkey(videoURI, windowStart)
	require.NoError(t, err)
	require.Contains(t, r, "-", "rkey is <windowTID>-<videoTID>")

	parts := strings.SplitN(r, "-", 2)
	require.Len(t, parts, 2)
	// Both halves should be parseable as TIDs (the joined whole is
	// not — atproto's TID is 13 chars exactly, the joined form has
	// 27).
	_, err = syntax.ParseTID(parts[0])
	require.NoError(t, err, "window half should be a valid TID")
	_, err = syntax.ParseTID(parts[1])
	require.NoError(t, err, "video half should be a valid TID")
	require.Equal(t, "3jw5xvr5gck2a", parts[1], "video half is the AT-URI's record key")

	// Same inputs → same rkey (idempotent overwrites on rerun).
	r2, err := viewCountRkey(videoURI, windowStart)
	require.NoError(t, err)
	require.Equal(t, r, r2)

	// Different window → different rkey.
	rLater, err := viewCountRkey(videoURI, windowStart.Add(5*time.Minute))
	require.NoError(t, err)
	require.NotEqual(t, r, rLater)
}

// fixtureMetafile returns a small two-track metafile suitable for
// driving the overlap math. Track 1 is video, track 2 is audio; both
// at timescale 1000 so 1 tick == 1 ms (makes test arithmetic trivial).
// Three segments per track, contiguous, video bytes laid out before
// audio bytes within each segment — same order the writer uses.
func fixtureMetafile() *vod.Metafile {
	return &vod.Metafile{
		BlobCID:  "bafyfixture",
		BlobSize: 1500,
		Tracks: map[string]vod.MetafileTrack{
			"1": {
				Type:      "video",
				Timescale: 1000,
				Segments: []vod.MetafileSegment{
					{Offset: 0, Size: 400, DurationTicks: 2000},   // [0..399] = 2s
					{Offset: 500, Size: 400, DurationTicks: 2000}, // [500..899] = 2s
					{Offset: 1000, Size: 400, DurationTicks: 2000},
				},
			},
			"2": {
				Type:      "audio",
				Timescale: 1000,
				Segments: []vod.MetafileSegment{
					{Offset: 400, Size: 100, DurationTicks: 2000}, // [400..499] = 2s
					{Offset: 900, Size: 100, DurationTicks: 2000}, // [900..999] = 2s
					{Offset: 1400, Size: 100, DurationTicks: 2000},
				},
			},
		},
	}
}

func TestRangeOverlapInTrack(t *testing.T) {
	meta := fixtureMetafile()
	video := meta.Tracks["1"]
	audio := meta.Tracks["2"]

	t.Run("exact segment", func(t *testing.T) {
		b, d := rangeOverlapInTrack(0, 399, video)
		require.Equal(t, int64(400), b)
		require.Equal(t, int64(2000), d, "full segment ⇒ full duration")
	})
	t.Run("half a segment", func(t *testing.T) {
		// First 200 bytes of segment 0 = half the bytes, half the duration.
		b, d := rangeOverlapInTrack(0, 199, video)
		require.Equal(t, int64(200), b)
		require.Equal(t, int64(1000), d, "byte-proportional duration credit")
	})
	t.Run("range spanning two video segments + gap", func(t *testing.T) {
		// [350..599]: last 50 of seg0 video + audio gap + first 100 of seg1 video.
		b, d := rangeOverlapInTrack(350, 599, video)
		require.Equal(t, int64(50+100), b)
		// 50/400 * 2000 + 100/400 * 2000 = 250 + 500 = 750ms.
		require.Equal(t, int64(750), d)
	})
	t.Run("range hits only audio offsets", func(t *testing.T) {
		b, _ := rangeOverlapInTrack(400, 499, video)
		require.Equal(t, int64(0), b, "audio bytes don't credit the video track")
		b, d := rangeOverlapInTrack(400, 499, audio)
		require.Equal(t, int64(100), b)
		require.Equal(t, int64(2000), d)
	})
	t.Run("whole blob fetch", func(t *testing.T) {
		// [0..1499] covers every segment in both tracks.
		b, d := rangeOverlapInTrack(0, 1499, video)
		require.Equal(t, int64(400*3), b)
		require.Equal(t, int64(2000*3), d)
		b, d = rangeOverlapInTrack(0, 1499, audio)
		require.Equal(t, int64(100*3), b)
		require.Equal(t, int64(2000*3), d)
	})
	t.Run("empty range", func(t *testing.T) {
		b, d := rangeOverlapInTrack(100, 50, video)
		require.Equal(t, int64(0), b)
		require.Equal(t, int64(0), d)
	})
	t.Run("zero timescale punts", func(t *testing.T) {
		bad := vod.MetafileTrack{Timescale: 0, Segments: []vod.MetafileSegment{{Offset: 0, Size: 100, DurationTicks: 1000}}}
		b, d := rangeOverlapInTrack(0, 99, bad)
		require.Equal(t, int64(0), b)
		require.Equal(t, int64(0), d)
	})
}

func TestAggregateWindowTrackUsage(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		cidA   = "bafyfixture"
		sid    = "session1"
	)

	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: sid})
	// Three segment_requests: video-seg0, audio-seg0, video-seg1.
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: cidA, SID: sid, RangeStart: 0, RangeEnd: 399})
	w.Log(ctx, Event{Ts: now.Add(2 * time.Second), Type: EventTypeSegmentRequest, CID: cidA, SID: sid, RangeStart: 400, RangeEnd: 499})
	w.Log(ctx, Event{Ts: now.Add(3 * time.Second), Type: EventTypeSegmentRequest, CID: cidA, SID: sid, RangeStart: 500, RangeEnd: 899})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	refs := fixtureTrackRefs()
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
		FetchMetafile: func(ctx context.Context, cid string) (*vod.Metafile, error) {
			require.Equal(t, cidA, cid)
			return fixtureMetafile(), nil
		},
		FetchTrackRefs: func(ctx context.Context, cid string) (map[string]*comatproto.RepoStrongRef, error) {
			require.Equal(t, cidA, cid)
			return refs, nil
		},
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
	require.Equal(t, 1, res.MetafilesLoaded, "metafile cache: one fetch even though three segment_requests share the CID")

	byURI := map[string]TrackUsage{}
	for _, t := range res.VideoCounts[0].Tracks {
		byURI[t.Track.Uri] = t
	}
	require.Equal(t,
		TrackUsage{Track: refs["1"], Bytes: 800, DurationMS: 4000},
		byURI[refs["1"].Uri], "two video segments fully fetched")
	require.Equal(t,
		TrackUsage{Track: refs["2"], Bytes: 100, DurationMS: 2000},
		byURI[refs["2"].Uri], "one audio segment fully fetched")
}

func TestAggregateWindowVideoWithUsageButNoCount(t *testing.T) {
	// Bytes flowed for a sid that never qualified as a view (e.g. an
	// orphan blob fetch from a previously-manifested sid that gets
	// thresholded out). The objective totals still ship.
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		cidA   = "bafyfixture"
	)
	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: "lurker"})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: cidA, SID: "lurker", RangeStart: 0, RangeEnd: 399})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	refs := fixtureTrackRefs()
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart:       now.Add(-time.Minute),
		WindowEnd:         now.Add(time.Minute),
		ThresholdSegments: 5, // far above the one segment fetched
		FetchMetafile: func(ctx context.Context, cid string) (*vod.Metafile, error) {
			return fixtureMetafile(), nil
		},
		FetchTrackRefs: func(ctx context.Context, cid string) (map[string]*comatproto.RepoStrongRef, error) {
			return refs, nil
		},
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, int64(0), res.VideoCounts[0].Count, "below threshold ⇒ no view")
	require.Len(t, res.VideoCounts[0].Tracks, 1, "but the bytes are still reported")
	require.Equal(t, refs["1"].Uri, res.VideoCounts[0].Tracks[0].Track.Uri)
	require.Equal(t, int64(400), res.VideoCounts[0].Tracks[0].Bytes)
}

func TestAggregateWindowTidWithoutStrongRefIsDropped(t *testing.T) {
	// A track that has a metafile entry but no place.stream.media.track
	// record (e.g. mid-publish, or the record was deleted) doesn't get
	// a usage row — we can't reference it. The sid view still counts.
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const (
		videoA = "at://did:plc:alice/place.stream.video/v1"
		cidA   = "bafyfixture"
	)
	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: "sid"})
	// Range hits both video (track "1") and audio (track "2"), but the
	// resolver only knows about track "1".
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: cidA, SID: "sid", RangeStart: 0, RangeEnd: 499})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	refs := fixtureTrackRefs()
	partial := map[string]*comatproto.RepoStrongRef{"1": refs["1"]}
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
		FetchMetafile: func(ctx context.Context, cid string) (*vod.Metafile, error) {
			return fixtureMetafile(), nil
		},
		FetchTrackRefs: func(ctx context.Context, cid string) (map[string]*comatproto.RepoStrongRef, error) {
			return partial, nil
		},
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
	require.Len(t, res.VideoCounts[0].Tracks, 1, "only the resolved track gets a row")
	require.Equal(t, refs["1"].Uri, res.VideoCounts[0].Tracks[0].Track.Uri)
}

func TestAggregateWindowMissingMetafileDoesNotPoison(t *testing.T) {
	// A segment_request whose CID has no metafile still counts toward
	// the sid view (the threshold cares about request count, not byte
	// math) — it just contributes zero bytes/duration.
	root := t.TempDir()
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w := newAggTestWriter(t, root, "did:web:node1", now)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const videoA = "at://did:plc:alice/place.stream.video/v1"
	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: videoA, SID: "sid"})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafymissing", SID: "sid", RangeStart: 0, RangeEnd: 100})
	require.NoError(t, w.Close())

	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	res, err := AggregateWindow(context.Background(), store, AggregateInput{
		WindowStart: now.Add(-time.Minute),
		WindowEnd:   now.Add(time.Minute),
		FetchMetafile: func(ctx context.Context, cid string) (*vod.Metafile, error) {
			return nil, nil // not found
		},
	})
	require.NoError(t, err)
	require.Len(t, res.VideoCounts, 1)
	require.Equal(t, int64(1), res.VideoCounts[0].Count)
	require.Empty(t, res.VideoCounts[0].Tracks, "no metafile ⇒ no per-track credit")
}
