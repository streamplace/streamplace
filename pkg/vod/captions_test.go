package vod

import (
	"bytes"
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func TestVideoCaptionsMuxlPrecedenceAndLazyTrack(t *testing.T) {
	ctx := context.Background()
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	data, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	ch := make(chan *upstream.Event, 32)
	errs := make(chan error, 1)
	go func() { errs <- eng.SegmentEvents(ctx, bytes.NewReader(data), ch); close(ch) }()
	var segments []*upstream.Event
	for ev := range ch {
		if ev.Type == "segment" {
			segments = append(segments, ev)
		}
	}
	require.NoError(t, <-errs)
	require.GreaterOrEqual(t, len(segments), 2)
	var joined []byte
	var wantStart, wantEnd time.Duration
	require.Zero(t, segments[0].FirstDecodeTimes["1"], "fixture video starts at zero")
	for i, ev := range segments[:2] {
		var seg []byte
		var keys []string
		for id := range ev.Tracks {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			seg = append(seg, ev.Tracks[id]...)
		}
		if i == 1 {
			// An empty text track gives SegmentClock the existing AV header.
			empty, err := eng.AddTextTrack(ctx, seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "auto"}, nil)
			require.NoError(t, err)
			var hdr bytes.Buffer
			require.NoError(t, eng.Wrap(ctx, bytes.NewReader(empty), "flat", &hdr))
			base, _, err := captions.SegmentClock(hdr.Bytes(), seg)
			require.NoError(t, err)
			wantStart = time.Duration(base.Milliseconds()+100) * time.Millisecond
			wantEnd = time.Duration(base.Milliseconds()+500) * time.Millisecond
			seg, err = eng.AddTextTrack(ctx, seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "auto"}, []upstream.TextCue{{ID: "archived", Text: "The mastered text", Start: uint64(base.Milliseconds() + 100), End: uint64(base.Milliseconds() + 500)}})
			require.NoError(t, err)
		}
		joined = append(joined, seg...)
	}
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	const cid = "captions-fixture"
	writer, err := store.NewWriter(ctx, BlobsPrefix+cid+".mp4", "video/mp4")
	require.NoError(t, err)
	_, err = writer.Write(joined)
	require.NoError(t, err)
	require.NoError(t, writer.Complete())
	require.NoError(t, writer.Close())
	mb := newFragmentMetafileBuilder(ctx, store)
	ch = make(chan *upstream.Event, 32)
	go func() { errs <- eng.UnwrapEvents(ctx, bytes.NewReader(joined), ch); close(ch) }()
	for ev := range ch {
		require.NoError(t, mb.Observe(ev))
	}
	require.NoError(t, <-errs)
	meta := mb.Finalize(cid, int64(len(joined)))
	require.NoError(t, writeMetafile(ctx, store, cid, meta))
	require.Equal(t, "text", meta.Tracks["9"].Type)
	require.Equal(t, []textProbeJSON{{TrackID: "9", Language: "en", Label: "auto"}}, metafileTextTracks(meta))
	m, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	const video = "at://did:plc:alice/place.stream.video/vod"
	const trackURI = "at://did:plc:alice/place.stream.media.track/track"
	require.NoError(t, m.UpsertMediaTrack(ctx, placestream.MediaTrack{Track: placestream.MediaTrack_Track{MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{Blob: cid, TrackId: "1", MediaType: "video"}}}, syntax.ATURI(trackURI)))
	require.NoError(t, m.UpsertVideo(ctx, placestream.Video{Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{Tracks: []comatproto.RepoStrongRef{{Uri: trackURI, Cid: "bafy"}}}}}, syntax.ATURI(video)))
	for _, source := range []string{"auto", "imported"} {
		rec := placestream.CaptionTranscript{Subject: comatproto.RepoStrongRef{Uri: video, Cid: "bafy"}, Text: "The mastered text", StartMs: 0, Timings: []int64{100, 100, 200}, Language: "en", Source: source, CreatedAt: "2026-09-30T00:00:00Z"}
		require.NoError(t, m.UpsertCaptionTranscript(ctx, rec, syntax.ATURI("at://did:plc:alice/place.stream.caption.transcript/"+source)))
	}
	p := &VideoCaptions{Model: m, Store: store, Records: &records.Provider{Store: m}}
	tracks, err := p.Tracks(ctx, video)
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	got, err := p.Cues(ctx, video, "canonical-auto-en")
	require.NoError(t, err)
	require.Equal(t, []captions.TimedCue{{ID: "archived", Text: "The mastered text", Start: wantStart, End: wantEnd}}, got)
	for _, track := range tracks {
		if track.Origin == captions.OriginRecord {
			require.Equal(t, captions.SourceImported, track.Source)
		}
	}
}
