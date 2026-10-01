package vod

import (
	"bytes"
	"context"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/captions"
)

func reconnectCaptionFixture(t *testing.T, changeLanguage, textFirst bool) (*videoCaptionView, time.Duration, error) {
	t.Helper()
	ctx := context.Background()
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	data, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	ch := make(chan *upstream.Event, 32)
	errs := make(chan error, 1)
	go func() { errs <- eng.SegmentEvents(ctx, bytes.NewReader(data), ch); close(ch) }()
	var source []*upstream.Event
	var scale uint32
	for ev := range ch {
		if ev.Catalog != nil && ev.Catalog.Video != nil {
			for _, c := range ev.Catalog.Video.Renditions {
				if c.TrackID() == 1 {
					scale = c.Timescale()
				}
			}
		}
		if ev.Type == "segment" {
			source = append(source, ev)
		}
	}
	require.NoError(t, <-errs)
	require.GreaterOrEqual(t, len(source), 2)
	require.NotZero(t, scale)
	var joined []byte
	for i, index := range []int{0, 1, 0} {
		ev := source[index]
		var part []byte
		for _, id := range []string{"1", "2"} {
			part = append(part, ev.Tracks[id]...)
		}
		start := captionTicks(ev.FirstDecodeTimes["1"], scale).Milliseconds() + 100
		lang, label, text, id := "en", "auto", "English speech", "english"
		if i == 1 {
			id = "english-two"
		}
		if i == 2 {
			id = "reconnected"
			if changeLanguage {
				lang, label, text = "es", "human", "Habla española"
			}
		}
		part, err = eng.AddTextTrack(ctx, part, upstream.TextTrack{TrackID: 100, Language: lang, Label: label}, []upstream.TextCue{{ID: id, Text: text, Start: uint64(start), End: uint64(start + 300)}})
		require.NoError(t, err)
		events := make(chan *upstream.Event, 8)
		go func() { errs <- eng.UnwrapEvents(ctx, bytes.NewReader(part), events); close(events) }()
		for event := range events {
			if event.Type != "segment" {
				continue
			}
			var keys []string
			for key := range event.Tracks {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				if textFirst && (keys[i] == "100" || keys[j] == "100") {
					return keys[i] == "100"
				}
				a, _ := strconv.ParseUint(keys[i], 10, 32)
				b, _ := strconv.ParseUint(keys[j], 10, 32)
				return a < b
			})
			for _, key := range keys {
				joined = append(joined, event.Tracks[key]...)
			}
		}
		require.NoError(t, <-errs)
	}
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	const sourceKey = "reconnect-source.m4s"
	w, err := store.NewWriter(ctx, sourceKey, "video/iso.segment")
	require.NoError(t, err)
	_, err = w.Write(joined)
	require.NoError(t, err)
	require.NoError(t, w.Complete())
	require.NoError(t, w.Close())
	cid, _, meta, err := hashAndBuildFragmentMetafile(ctx, store, []string{sourceKey})
	want := captionTicks(source[0].Durations["1"]+source[1].Durations["1"], scale) + 100*time.Millisecond
	if textFirst {
		require.Nil(t, meta, "non-canonical input must not emit mis-indexed byte ranges")
		return nil, want, err
	}
	if err != nil {
		return nil, want, err
	}
	w, err = store.NewWriter(ctx, BlobsPrefix+cid+".mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write(joined)
	require.NoError(t, err)
	require.NoError(t, w.Complete())
	require.NoError(t, w.Close())
	require.NoError(t, writeMetafile(ctx, store, cid, meta))
	out := &videoCaptionView{cues: map[string][]captions.TimedCue{}}
	err = (&VideoCaptions{Store: store}).readMuxl(ctx, cid, "at://did:plc:alice/place.stream.video/vod", out)
	return out, want, err
}

func TestVideoCaptionsReconnectKeepsConfigurationEpochs(t *testing.T) {
	out, want, err := reconnectCaptionFixture(t, true, false)
	require.NoError(t, err)
	english := out.cues["canonical-auto-en"]
	spanish := out.cues["canonical-human-es"]
	require.Len(t, english, 2, "a reused numeric ID must not relabel earlier English speech")
	require.Equal(t, "English speech", english[0].Text)
	require.Equal(t, []captions.TimedCue{{ID: "reconnected", Text: "Habla española", Start: want, End: want + 300*time.Millisecond}}, spanish)
}

func TestVideoCaptionsReconnectUsesContainingGoPClock(t *testing.T) {
	out, want, err := reconnectCaptionFixture(t, false, false)
	require.NoError(t, err)
	cues := out.cues["canonical-auto-en"]
	require.Len(t, cues, 3)
	require.Equal(t, want, cues[2].Start, "reset text uses its containing GoP's accumulated VOD clock")
}

func TestVideoCaptionIndexerRejectsNoncanonicalTextFirstOrder(t *testing.T) {
	_, _, err := reconnectCaptionFixture(t, false, true)
	require.ErrorContains(t, err, "non-canonical MUXL track order")
}
