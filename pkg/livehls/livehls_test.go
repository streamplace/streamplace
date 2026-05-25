package livehls

import (
	"bytes"
	"strings"
	"testing"

	"stream.place/streamplace/pkg/muxl"
)

func initEvent() *muxl.MuxlEvent {
	return &muxl.MuxlEvent{
		Type:       "init",
		Data:       []byte("INIT"), // 4 bytes — stands in for ftyp+moov
		TrackInits: map[string][]byte{"1": []byte("VINIT"), "2": []byte("AINIT")},
		Catalog: &muxl.MuxlCatalog{
			Video: &muxl.MuxlCatalogVideo{Renditions: map[string]muxl.MuxlVideoConfig{
				"v": {Codec: "avc1.640028", Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 90000, TrackID: 1}, CodedWidth: 1280, CodedHeight: 720},
			}},
			Audio: &muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{
				"a": {Codec: "mp4a.40.2", Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 48000, TrackID: 2}, SampleRate: 48000, NumberOfChannels: 2},
			}},
		},
	}
}

func segEvent(v, a []byte) *muxl.MuxlEvent {
	return &muxl.MuxlEvent{
		Type:         "segment",
		Tracks:       map[string][]byte{"1": v, "2": a},
		Durations:    map[string]uint64{"1": 90000, "2": 48000}, // 1s each
		SampleCounts: map[string]uint32{"1": 30, "2": 50},
	}
}

func TestWriterAppendsVerbatimAndIndexes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Observe(initEvent()); err != nil {
		t.Fatal(err)
	}
	v1 := bytes.Repeat([]byte{0xAA}, 100)
	a1 := bytes.Repeat([]byte{0xBB}, 40)
	v2 := bytes.Repeat([]byte{0xCC}, 110)
	a2 := bytes.Repeat([]byte{0xDD}, 42)
	if err := w.Observe(segEvent(v1, a1)); err != nil {
		t.Fatal(err)
	}
	if err := w.Observe(segEvent(v2, a2)); err != nil {
		t.Fatal(err)
	}

	// fMP4 = INIT + (v1,a1) + (v2,a2): per-segment tracks in sorted id order.
	want := bytes.Join([][]byte{[]byte("INIT"), v1, a1, v2, a2}, nil)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("fmp4 bytes mismatch: got %d bytes, want %d", buf.Len(), len(want))
	}

	// Track 1 (video): seg0 @ len("INIT")=4, seg1 @ 4+100+40=144.
	tr1 := w.Track("1")
	if tr1.Type != "video" || tr1.Width != 1280 || tr1.Height != 720 {
		t.Errorf("track1 config wrong: %+v", tr1)
	}
	if len(tr1.Segments) != 2 ||
		tr1.Segments[0] != (Segment{Offset: 4, Size: 100, DurationTicks: 90000, SampleCount: 30}) ||
		tr1.Segments[1] != (Segment{Offset: 144, Size: 110, DurationTicks: 90000, SampleCount: 30}) {
		t.Errorf("track1 segments wrong: %+v", tr1.Segments)
	}

	// Track 2 (audio): seg0 @ 4+100=104, seg1 @ 144+110=254.
	tr2 := w.Track("2")
	if tr2.Type != "audio" || tr2.SampleRate != 48000 || tr2.Channels != 2 {
		t.Errorf("track2 config wrong: %+v", tr2)
	}
	if tr2.Segments[0].Offset != 104 || tr2.Segments[0].Size != 40 ||
		tr2.Segments[1].Offset != 254 || tr2.Segments[1].Size != 42 {
		t.Errorf("track2 segments wrong: %+v", tr2.Segments)
	}
}

func TestMediaPlaylistLiveThenFinalize(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	_ = w.Observe(initEvent())
	_ = w.Observe(segEvent(bytes.Repeat([]byte{1}, 100), bytes.Repeat([]byte{2}, 40)))

	pl := w.MediaPlaylist("1", "init1.mp4", "blob.fmp4")
	for _, want := range []string{
		`#EXT-X-MAP:URI="init1.mp4"`,
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXTINF:1.000000,",
		"#EXT-X-BYTERANGE:100@4",
		"blob.fmp4",
	} {
		if !strings.Contains(pl, want) {
			t.Errorf("live playlist missing %q:\n%s", want, pl)
		}
	}
	if strings.Contains(pl, "#EXT-X-ENDLIST") {
		t.Error("a live playlist must not carry EXT-X-ENDLIST")
	}

	w.Finalize()
	pl = w.MediaPlaylist("1", "init1.mp4", "blob.fmp4")
	if !strings.Contains(pl, "#EXT-X-ENDLIST") || !strings.Contains(pl, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Errorf("finalized playlist must be a VOD playlist with ENDLIST:\n%s", pl)
	}
}

func TestSlidingWindowEvictsAndAdvancesMediaSequence(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithWindow(2))
	_ = w.Observe(initEvent())
	for i := 0; i < 4; i++ {
		_ = w.Observe(segEvent([]byte{byte(i)}, []byte{byte(i)}))
	}
	tr := w.Track("1")
	if len(tr.Segments) != 2 {
		t.Fatalf("window=2 should cap the playlist at 2 segments, got %d", len(tr.Segments))
	}
	// 4 emitted, 2 retained → 2 evicted → media sequence 2.
	if !strings.Contains(w.MediaPlaylist("1", "i", "b"), "#EXT-X-MEDIA-SEQUENCE:2") {
		t.Errorf("expected EXT-X-MEDIA-SEQUENCE:2 after evicting 2 segments")
	}
	// Bytes are never truncated: all 4 segments (both tracks) plus init remain.
	if buf.Len() != len("INIT")+4*2 {
		t.Errorf("underlying fmp4 should retain all bytes, got %d", buf.Len())
	}
}

func TestMasterPlaylist(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	_ = w.Observe(initEvent())
	_ = w.Observe(segEvent(bytes.Repeat([]byte{1}, 100), bytes.Repeat([]byte{2}, 40)))

	m := w.MasterPlaylist(func(tid string) string { return "track" + tid + ".m3u8" })
	for _, want := range []string{
		"#EXT-X-MEDIA:TYPE=AUDIO",
		`GROUP-ID="audio"`,
		"#EXT-X-STREAM-INF:",
		"RESOLUTION=1280x720",
		"track1.m3u8",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("master playlist missing %q:\n%s", want, m)
		}
	}
}
