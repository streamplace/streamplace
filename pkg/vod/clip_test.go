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

// TestFindClip clips across completed recordings and the in-memory buffer,
// which overlap the way they do on a recording node (the buffer still holds
// segments a completed object also has). A clip must take each segment once,
// in order, cut a recording at the right byte offsets, and come out as muxl's
// flat MP4 of exactly the segments starting in range.
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
	// record stores segs as one completed object of did's stream, starting at at(0).
	record := func(did string, segs [][]byte) {
		key := "live-rec/" + did + "/a.m4s"
		body := bytes.Join(segs, nil)
		require.NoError(t, writeBlob(ctx, store, key, body))
		id, err := state.RecordStart(ctx, did, "bucket", key, "at://"+did, at(0))
		require.NoError(t, err)
		var index []s3.RecordedSegment
		for i, seg := range segs {
			index = append(index, s3.RecordedSegment{StartMs: at(i).UnixMilli(), Size: int64(len(seg))})
		}
		require.NoError(t, state.RecordComplete(ctx, id, 1, int64(len(body)), index))
	}
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
	record("did:plc:mixed", segs[:1])
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
	record("did:plc:recorded", segs)
	require.Equal(t, flatOracle(t, segs[1:]), clipBytes("did:plc:recorded", at(1).Add(-time.Millisecond), at(2), fakeBuffer{}))
}

// TestFindClipOpenRecording covers a node that doesn't hold the object still
// being recorded (another node ingests the stream and this one only buffers
// the last few minutes): a clip reaching into the gap must fail rather than
// silently skip it, while one this node fully holds still succeeds.
func TestFindClipOpenRecording(t *testing.T) {
	ctx := context.Background()
	segs := liveSegments(t)
	const did = "did:plc:clip"
	now := time.Now()
	state, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	_, err = state.RecordStart(ctx, did, "bucket", "live-rec/"+did+"/open.m4s", "at://ls", now.Add(-5*time.Minute))
	require.NoError(t, err)
	buf := fakeBuffer{{Start: now.Add(-time.Minute), Published: true, Data: segs[0]}}

	_, err = FindClip(ctx, state, store, buf, did, now.Add(-3*time.Minute), now, false)
	require.ErrorIs(t, err, ErrClipUnavailable)
	_, err = FindClip(ctx, state, store, buf, did, now.Add(-time.Minute), now, false)
	require.NoError(t, err)

	// The recording node holds the whole open object in memory.
	held := fakeBuffer{
		{Start: now.Add(-5 * time.Minute), Published: true, Data: segs[0]},
		{Start: now.Add(-2 * time.Minute), Published: true, Data: segs[1]},
	}
	_, err = FindClip(ctx, state, store, held, did, now.Add(-3*time.Minute), now, false)
	require.NoError(t, err)
}
