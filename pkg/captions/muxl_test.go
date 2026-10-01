package captions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
)

func TestCanonicalMuxlClockAndContinuation(t *testing.T) {
	ctx := context.Background()
	eng, err := upstream.NewWASM(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, eng.Close(ctx)) })
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../test/fixtures/h264-opus-frag.mp4"))
	require.NoError(t, err)
	ch := make(chan *upstream.Event, 32)
	errCh := make(chan error, 1)
	go func() { errCh <- eng.SegmentEvents(ctx, bytes.NewReader(data), ch); close(ch) }()
	var segments []*upstream.Event
	for ev := range ch {
		if ev.Type == "segment" {
			segments = append(segments, ev)
		}
	}
	require.NoError(t, <-errCh)
	require.GreaterOrEqual(t, len(segments), 2)
	join := func(ev *upstream.Event) []byte {
		keys := make([]string, 0, len(ev.Tracks))
		for id := range ev.Tracks {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		var out []byte
		for _, id := range keys {
			out = append(out, ev.Tracks[id]...)
		}
		return out
	}
	var plainHeader bytes.Buffer
	plain := join(segments[0])
	require.NoError(t, eng.Wrap(ctx, bytes.NewReader(plain), "flat", &plainHeader))
	_, has, err := SegmentClock(plainHeader.Bytes(), plain)
	require.NoError(t, err)
	require.False(t, has)
	track := upstream.TextTrack{TrackID: 9, Language: "en", Label: "auto"}
	// Discover actual media boundaries with a gap-covering track: the fixture
	// deliberately starts away from zero, so receive-time/zero-based math fails.
	withEmpty, err := eng.AddTextTrack(ctx, plain, track, nil)
	require.NoError(t, err)
	var header bytes.Buffer
	require.NoError(t, eng.Wrap(ctx, bytes.NewReader(withEmpty), "flat", &header))
	media0, has, err := SegmentClock(header.Bytes(), plain)
	require.NoError(t, err)
	require.True(t, has)
	plain1 := join(segments[1])
	withEmpty1, err := eng.AddTextTrack(ctx, plain1, track, nil)
	require.NoError(t, err)
	var header1 bytes.Buffer
	require.NoError(t, eng.Wrap(ctx, bytes.NewReader(withEmpty1), "flat", &header1))
	media1, _, err := SegmentClock(header1.Bytes(), plain1)
	require.NoError(t, err)
	cue := upstream.TextCue{ID: "cross", Text: "Across the boundary", Start: uint64(media0.Milliseconds() + 250), End: uint64(media1.Milliseconds() + 500)}
	wall := time.Unix(1700000000, 0).UTC()
	hub := NewHub(0)
	for i, seg := range [][]byte{plain, plain1} {
		added, err := eng.AddTextTrack(ctx, seg, track, []upstream.TextCue{cue})
		require.NoError(t, err)
		var hdr bytes.Buffer
		require.NoError(t, eng.Wrap(ctx, bytes.NewReader(added), "flat", &hdr))
		start := wall
		if i == 1 {
			start = start.Add(media1 - media0)
		}
		media, has, err := SegmentClock(hdr.Bytes(), added)
		require.NoError(t, err)
		require.True(t, has)
		events, err := ReadCanonicalWithClock(ctx, added, media, start, "did:plc:alice")
		require.NoError(t, err)
		for _, ev := range events {
			hub.PublishCanonical("alice", ev.Track, ev.Cue)
			hub.PublishCanonical("alice", ev.Track, ev.Cue)
		}
	}
	got := hub.Cues("alice", "canonical-auto-en", wall, wall.Add(time.Minute))
	require.Len(t, got, 1)
	require.Equal(t, "Across the boundary", got[0].Text)
	require.Equal(t, wall.Add(time.Duration(cue.Start)*time.Millisecond-media0), got[0].Start)
	require.Equal(t, wall.Add(time.Duration(cue.End)*time.Millisecond-media0), got[0].End)
	// A lazy second language/source and an empty earlier track are independently
	// discovered; no empty cue is published.
	second, err := eng.AddTextTrack(ctx, withEmpty1, upstream.TextTrack{TrackID: 10, Language: "es", Label: "human"}, []upstream.TextCue{{ID: "es", Text: "Hola", Start: uint64(media1.Milliseconds() + 100), End: uint64(media1.Milliseconds() + 400)}})
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(ctx, bytes.NewReader(second), "flat", &hdr))
	media, has, err := SegmentClock(hdr.Bytes(), second)
	require.NoError(t, err)
	require.True(t, has)
	events, err := ReadCanonicalWithClock(ctx, second, media, wall.Add(media1-media0), "did:plc:alice")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "canonical-human-es", events[0].Track.ID)
	require.Equal(t, "Hola", events[0].Cue.Text)
}
