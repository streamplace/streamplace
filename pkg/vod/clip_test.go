package vod

import (
	"bytes"
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
)

type fakeBuffer []media.BufferedSegment

func (b fakeBuffer) BufferedSegments(string) []media.BufferedSegment { return b }

// liveSegments signs the fragmented fixture the way live ingest does and
// returns each GoP's bare canonical segment, in order.
func liveSegments(t *testing.T) [][]byte {
	t.Helper()
	ms, err := media.NewEphemeralMediaSigner("test-streamer")
	require.NoError(t, err)
	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	events := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- ms.SignSegmentStream(context.Background(), bytes.NewReader(frag), events)
		close(events)
	}()
	var segs [][]byte
	for ev := range events {
		if ev.Type != "signed-segment" {
			continue
		}
		tids := make([]string, 0, len(ev.Tracks))
		for tid := range ev.Tracks {
			tids = append(tids, tid)
		}
		sort.Strings(tids)
		var seg []byte
		for _, tid := range tids {
			seg = append(seg, ev.Tracks[tid]...)
		}
		segs = append(segs, seg)
	}
	require.NoError(t, <-errCh)
	return segs
}

// flatOracle is muxl's own flat MP4 of the segments: what a clip of exactly
// those segments must be, byte for byte.
func flatOracle(t *testing.T, segs [][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(context.Background(), bytes.NewReader(bytes.Join(segs, nil)), "flat", &out))
	return out.Bytes()
}

// recordObject stores segs back to back as one completed recording object of
// did's stream, indexed with the given start times.
func recordObject(t *testing.T, state *statedb.StatefulDB, store blob.Store, did, key string, segs [][]byte, starts []time.Time) {
	t.Helper()
	ctx := context.Background()
	body := bytes.Join(segs, nil)
	require.NoError(t, writeBlob(ctx, store, key, body))
	id, err := state.RecordStart(ctx, did, "bucket", key, "at://"+did, starts[0])
	require.NoError(t, err)
	var index []s3.RecordedSegment
	for i, seg := range segs {
		index = append(index, s3.RecordedSegment{StartMs: starts[i].UnixMilli(), Size: int64(len(seg))})
	}
	require.NoError(t, state.RecordComplete(ctx, id, 1, int64(len(body)), index))
}

// TestFindClip clips across completed recordings and the in-memory buffer,
// which overlap the way they do on a recording node (the buffer still holds
// segments a completed object also has). A clip must take each segment once,
// in start-time order, cut a recording at the right byte offsets, and come
// out as muxl's flat MP4 of exactly the segments starting in range.
func TestFindClip(t *testing.T) {
	ctx := context.Background()
	segs := liveSegments(t)
	require.Len(t, segs, 2, "fixture GoPs")

	t0 := time.UnixMilli(1_760_000_000_000)
	at := func(i int) time.Time { return t0.Add(time.Duration(i) * 2 * time.Second) }

	state, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	clipBytes := func(did string, from, to time.Time, b SegmentBuffer) []byte {
		t.Helper()
		clip, err := FindClip(ctx, state, store, b, did, from, to, false)
		require.NoError(t, err)
		var out bytes.Buffer
		require.NoError(t, clip.WriteMP4(ctx, &out))
		header, err := clip.Header(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(len(header))+clip.BodySize(), int64(out.Len()), "advertised length")
		return out.Bytes()
	}

	// Segment 0 is in a completed recording; memory still holds it, plus
	// segment 1, which is newer than any recording.
	recordObject(t, state, store, "did:plc:mixed", "mixed.m4s", segs[:1], []time.Time{at(0)})
	buf := fakeBuffer{
		{Start: at(0), Published: true, Data: segs[0]},
		{Start: at(1), Published: true, Data: segs[1]},
	}
	require.Equal(t, flatOracle(t, segs), clipBytes("did:plc:mixed", at(0), at(1), buf), "recording + memory, each segment once")

	// Pre-live segments in memory stay out of a public clip.
	prelive := fakeBuffer{{Start: at(1), Published: false, Data: segs[1]}}
	require.Equal(t, flatOracle(t, segs[:1]), clipBytes("did:plc:mixed", at(0), at(1), prelive))
	_, err = FindClip(ctx, state, store, prelive, "did:plc:mixed", at(1), at(2), false)
	require.ErrorIs(t, err, ErrClipNotFound)

	// A range starting inside a recording is cut out of the object's middle.
	recordObject(t, state, store, "did:plc:recorded", "recorded.m4s", segs, []time.Time{at(0), at(1)})
	require.Equal(t, flatOracle(t, segs[1:]), clipBytes("did:plc:recorded", at(1).Add(-time.Millisecond), at(2), fakeBuffer{}))

	// The recorder appends segments in arrival order, which can differ from
	// start-time order; the clip is still in start-time order, and a range
	// ending before the first-queued segment still finds the earlier one.
	recordObject(t, state, store, "did:plc:reordered", "reordered.m4s", [][]byte{segs[1], segs[0]}, []time.Time{at(1), at(0)})
	require.Equal(t, flatOracle(t, segs), clipBytes("did:plc:reordered", at(0), at(1), fakeBuffer{}))
	require.Equal(t, flatOracle(t, segs[:1]), clipBytes("did:plc:reordered", at(0), at(0), fakeBuffer{}))
}

// TestFindClipOpenRecording covers the object still being recorded, which
// only the ingesting node holds (in memory): a clip reaching into it must
// fail on a node that can't vouch for every segment, rather than silently
// return a clip with a hole.
func TestFindClipOpenRecording(t *testing.T) {
	ctx := context.Background()
	segs := liveSegments(t)
	const did = "did:plc:clip"
	now := time.Now()
	state, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	_, err = state.RecordStart(ctx, did, "bucket", "open.m4s", "at://ls", now.Add(-5*time.Minute))
	require.NoError(t, err)
	clip := func(buf fakeBuffer, from, to time.Time) error {
		_, err := FindClip(ctx, state, store, buf, did, from, to, false)
		return err
	}
	ago := func(m time.Duration) time.Time { return now.Add(-m * time.Minute) }

	// The ingesting node holds the whole open object in memory.
	ingest := fakeBuffer{
		{Start: ago(5), Published: true, Local: true, Data: segs[0]},
		{Start: ago(2), Published: true, Local: true, Data: segs[1]},
	}
	require.NoError(t, clip(ingest, ago(3), now))
	// A node holding only the last two minutes can clip just those.
	require.ErrorIs(t, clip(ingest[1:], ago(3), now), ErrClipUnavailable)
	require.NoError(t, clip(ingest[1:], ago(2), now))
	// Replicated segments may have gaps between them, however far back they reach.
	replica := fakeBuffer{
		{Start: ago(5), Published: true, Data: segs[0]},
		{Start: ago(2), Published: true, Data: segs[1]},
	}
	require.ErrorIs(t, clip(replica, ago(3), now), ErrClipUnavailable)
	// A moderation clip takes what the replica has instead.
	_, err = FindClip(ctx, state, store, replica, did, ago(3), now, true)
	require.NoError(t, err)
	// Video after the range doesn't decide it: here the streamer moved to
	// another ingest node, whose segments now reach this one replicated.
	moved := fakeBuffer{
		{Start: ago(5), Published: true, Local: true, Data: segs[0]},
		{Start: ago(1), Published: true, Data: segs[1]},
	}
	require.NoError(t, clip(moved, ago(5), ago(2)))
	require.ErrorIs(t, clip(moved, ago(5), now), ErrClipUnavailable)
}
