package livehls

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRunProducesLiveHLSFromFmp4 drives a real fMP4 through the muxl wasm
// segmenter and the live HLS writer, then checks that the growing fMP4 and
// the per-track byte-range index/playlists are internally consistent.
func TestRunProducesLiveHLSFromFmp4(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fmp4, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/h264-opus-frag.mp4"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var out bytes.Buffer
	w, err := Run(context.Background(), bytes.NewReader(fmp4), &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("no fMP4 bytes written")
	}
	// The growing fMP4 must begin with the init segment (ftyp box).
	if out.Len() < 8 || string(out.Bytes()[4:8]) != "ftyp" {
		t.Fatalf("output fMP4 should start with an ftyp box, got %q", firstFourCC(out.Bytes()))
	}

	tids := w.TrackIDs()
	if len(tids) == 0 {
		t.Fatal("no tracks produced")
	}

	totalSegs := 0
	for _, tid := range tids {
		tr := w.Track(tid)
		for _, s := range tr.Segments {
			// Every indexed byte range must lie within the bytes we appended.
			if s.Size <= 0 || s.Offset < int64(len("ftyp")) || s.Offset+s.Size > int64(out.Len()) {
				t.Errorf("track %s segment byte range out of bounds: %+v (fmp4 %d bytes)", tid, s, out.Len())
			}
			totalSegs++
		}
		pl := w.MediaPlaylist(tid, "init.mp4", "blob.fmp4")
		for _, want := range []string{"#EXTM3U", "#EXT-X-MAP:", "#EXT-X-BYTERANGE:"} {
			if !strings.Contains(pl, want) {
				t.Errorf("track %s media playlist missing %q:\n%s", tid, want, pl)
			}
		}
		// Live (not finalized) → no ENDLIST yet.
		if strings.Contains(pl, "#EXT-X-ENDLIST") {
			t.Errorf("live playlist should not have ENDLIST before Finalize")
		}
	}
	if totalSegs == 0 {
		t.Fatal("no segments indexed")
	}

	m := w.MasterPlaylist(func(tid string) string { return "t" + tid + ".m3u8" })
	if !strings.HasPrefix(m, "#EXTM3U") {
		t.Errorf("master playlist malformed:\n%s", m)
	}

	t.Logf("live HLS: %d tracks, %d segments, %d fMP4 bytes", len(tids), totalSegs, out.Len())
}

func firstFourCC(b []byte) string {
	if len(b) >= 8 {
		return string(b[4:8])
	}
	return ""
}
