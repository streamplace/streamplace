package livehls

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

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

func segURI(seq uint64) string { return fmt.Sprintf("seg%d.m4s", seq) }

func TestWriterStoresSegmentsInMemory(t *testing.T) {
	w := NewWriter()
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

	// Track 1 (video): two segments, media sequences 0 and 1.
	tr1 := w.Track("1")
	if tr1.Type != "video" || tr1.Width != 1280 || tr1.Height != 720 {
		t.Errorf("track1 config wrong: %+v", tr1)
	}
	if len(tr1.Segments) != 2 {
		t.Fatalf("track1 should have 2 segments, got %d", len(tr1.Segments))
	}
	if tr1.Segments[0].Seq != 0 || tr1.Segments[0].Size() != 100 ||
		tr1.Segments[0].DurationTicks != 90000 || tr1.Segments[0].SampleCount != 30 {
		t.Errorf("track1 seg0 wrong: %+v", tr1.Segments[0])
	}
	if tr1.Segments[1].Seq != 1 || tr1.Segments[1].Size() != 110 {
		t.Errorf("track1 seg1 wrong: %+v", tr1.Segments[1])
	}
	// Segment bytes are served verbatim from memory, addressed by media seq.
	if !bytes.Equal(w.SegmentData("1", 0), v1) || !bytes.Equal(w.SegmentData("1", 1), v2) {
		t.Errorf("track1 SegmentData did not return the verbatim segment bytes")
	}
	if string(w.InitSegment("1")) != "VINIT" {
		t.Errorf("track1 init = %q, want VINIT", w.InitSegment("1"))
	}

	// Track 2 (audio).
	tr2 := w.Track("2")
	if tr2.Type != "audio" || tr2.SampleRate != 48000 || tr2.Channels != 2 {
		t.Errorf("track2 config wrong: %+v", tr2)
	}
	if !bytes.Equal(w.SegmentData("2", 0), a1) || !bytes.Equal(w.SegmentData("2", 1), a2) {
		t.Errorf("track2 SegmentData did not return the verbatim segment bytes")
	}
}

func TestMediaPlaylistLiveThenFinalize(t *testing.T) {
	w := NewWriter()
	_ = w.Observe(initEvent())
	_ = w.Observe(segEvent(bytes.Repeat([]byte{1}, 100), bytes.Repeat([]byte{2}, 40)))

	pl := w.MediaPlaylist("1", "init1.mp4", segURI)
	for _, want := range []string{
		`#EXT-X-MAP:URI="init1.mp4"`,
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXTINF:1.000000,",
		"seg0.m4s",
	} {
		if !strings.Contains(pl, want) {
			t.Errorf("live playlist missing %q:\n%s", want, pl)
		}
	}
	if strings.Contains(pl, "#EXT-X-ENDLIST") {
		t.Error("a live playlist must not carry EXT-X-ENDLIST")
	}

	w.Finalize()
	pl = w.MediaPlaylist("1", "init1.mp4", segURI)
	if !strings.Contains(pl, "#EXT-X-ENDLIST") || !strings.Contains(pl, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Errorf("finalized playlist must be a VOD playlist with ENDLIST:\n%s", pl)
	}
}

func TestSlidingWindowEvictsAndAdvancesMediaSequence(t *testing.T) {
	w := NewWriter(WithWindow(2))
	_ = w.Observe(initEvent())
	for i := 0; i < 4; i++ {
		_ = w.Observe(segEvent([]byte{byte(i)}, []byte{byte(i)}))
	}
	tr := w.Track("1")
	if len(tr.Segments) != 2 {
		t.Fatalf("window=2 should cap the window at 2 segments, got %d", len(tr.Segments))
	}
	// 4 emitted, 2 retained → media sequences 2 and 3 remain.
	if tr.Segments[0].Seq != 2 || tr.Segments[1].Seq != 3 {
		t.Errorf("expected retained seqs [2,3], got [%d,%d]", tr.Segments[0].Seq, tr.Segments[1].Seq)
	}
	if !strings.Contains(w.MediaPlaylist("1", "i", segURI), "#EXT-X-MEDIA-SEQUENCE:2") {
		t.Errorf("expected EXT-X-MEDIA-SEQUENCE:2 after evicting 2 segments")
	}
	// Evicted segments are gone from memory; retained ones are served.
	if w.SegmentData("1", 0) != nil {
		t.Errorf("evicted segment 0 should no longer be retained")
	}
	if w.SegmentData("1", 3) == nil {
		t.Errorf("segment 3 should still be retained")
	}
}

func TestRetentionEvictsStaleSegments(t *testing.T) {
	w := NewWriter(WithWindow(12), WithRetention(30*time.Second))
	clk := time.Unix(1000, 0)
	w.now = func() time.Time { return clk }

	if err := w.Observe(initEvent()); err != nil {
		t.Fatal(err)
	}
	_ = w.Observe(segEvent([]byte{1}, []byte{1})) // seq 0 @ t=1000
	clk = clk.Add(20 * time.Second)
	_ = w.Observe(segEvent([]byte{2}, []byte{2})) // seq 1 @ t=1020

	// t=1035, cutoff t=1005: seg0 (t=1000) ages out, seg1 (t=1020) survives.
	clk = time.Unix(1035, 0)
	if w.Empty() {
		t.Fatal("window with a fresh segment must not be empty")
	}
	tr := w.Track("1")
	if len(tr.Segments) != 1 || tr.Segments[0].Seq != 1 {
		t.Fatalf("want only seg1 retained, got %d segments", len(tr.Segments))
	}
	if w.SegmentData("1", 0) != nil {
		t.Error("aged-out seg0 must not be served")
	}

	// t=1060: both past retention (a stalled/ended stream) → window empties, so
	// a retrying player stops replaying the stale tail.
	clk = time.Unix(1060, 0)
	if !w.Empty() {
		t.Error("all segments past retention → window must be empty")
	}
	if pl := w.MediaPlaylist("1", "i", segURI); strings.Contains(pl, "seg1.m4s") {
		t.Errorf("aged-out segment must not appear in the playlist:\n%s", pl)
	}
}

func TestMasterPlaylist(t *testing.T) {
	w := NewWriter()
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
