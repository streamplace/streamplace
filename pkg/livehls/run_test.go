package livehls

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"stream.place/streamplace/pkg/muxl"
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

// TestRunSignedProducesSignedLiveHLS drives the same fixture through the
// signing segmenter and checks that each appended segment is a signed m4s —
// i.e. its byte range begins with a uuid box (the c2pa/S2PA prefix). Skips if
// the sibling muxl repo's test keys aren't present.
func TestRunSignedProducesSignedLiveHLS(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	muxlRoot := filepath.Join(repoRoot, "..", "muxl")

	keyPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-key.pem"))
	if err != nil {
		t.Skipf("skipping; sibling muxl test keys not available at %s: %v", muxlRoot, err)
	}
	certPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-cert.pem"))
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	fmp4, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/h264-opus-frag.mp4"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	manifest := []byte(`{"title":"livehls signed test","assertions":[{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}}]}`)

	var out bytes.Buffer
	w, err := RunSigned(context.Background(), bytes.NewReader(fmp4), muxl.SignerInput{
		CertPEM:         certPEM,
		KeyPEM:          keyPEM,
		TrackManifest:   manifest,
		WrapperManifest: manifest,
	}, &out)
	if err != nil {
		t.Fatalf("RunSigned: %v", err)
	}

	body := out.Bytes()
	signedSegs := 0
	for _, tid := range w.TrackIDs() {
		for _, s := range w.Track(tid).Segments {
			if s.Offset+s.Size > int64(len(body)) {
				t.Fatalf("segment out of range: %+v (%d bytes)", s, len(body))
			}
			// A signed canonical segment leads with the c2pa uuid box.
			if got := string(body[s.Offset+4 : s.Offset+8]); got != "uuid" {
				t.Errorf("track %s segment should start with a uuid box, got %q", tid, got)
			}
			signedSegs++
		}
	}
	if signedSegs == 0 {
		t.Fatal("no signed segments produced")
	}
	t.Logf("signed live HLS: %d segments, %d fMP4 bytes", signedSegs, len(body))
}
