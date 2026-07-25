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
// segmenter into the in-memory live HLS window, then checks the per-track
// segments and playlists are internally consistent.
func TestRunProducesLiveHLSFromFmp4(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fmp4, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/h264-opus-frag.mp4"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	w, err := Run(context.Background(), bytes.NewReader(fmp4))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	tids := w.TrackIDs()
	if len(tids) == 0 {
		t.Fatal("no tracks produced")
	}

	totalSegs := 0
	for _, tid := range tids {
		tr := w.Track(tid)
		for _, s := range tr.Segments {
			if s.Size() <= 0 {
				t.Errorf("track %s segment %d is empty", tid, s.Seq)
			}
			if w.SegmentData(tid, s.Seq) == nil {
				t.Errorf("track %s segment %d not retrievable by media sequence", tid, s.Seq)
			}
			totalSegs++
		}
		pl := w.MediaPlaylist(tid, "init.mp4", segURI)
		for _, want := range []string{"#EXTM3U", "#EXT-X-MAP:", "#EXTINF:"} {
			if !strings.Contains(pl, want) {
				t.Errorf("track %s media playlist missing %q:\n%s", tid, want, pl)
			}
		}
		if strings.Contains(pl, "#EXT-X-ENDLIST") {
			t.Errorf("live playlist should not have ENDLIST before Finalize")
		}
	}
	if totalSegs == 0 {
		t.Fatal("no segments produced")
	}

	m := w.MasterPlaylist(func(tid string) string { return "t" + tid + ".m3u8" })
	if !strings.HasPrefix(m, "#EXTM3U") {
		t.Errorf("master playlist malformed:\n%s", m)
	}

	t.Logf("live HLS: %d tracks, %d segments", len(tids), totalSegs)
}

// TestRunSignedProducesSignedLiveHLS drives the same fixture through the
// signing segmenter and checks each windowed segment is a signed m4s — i.e.
// its bytes begin with a uuid box (the c2pa/S2PA prefix). Skips if the sibling
// muxl repo's test keys aren't present.
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

	w, err := RunSigned(context.Background(), bytes.NewReader(fmp4), muxl.SignerInput{
		CertPEM:         certPEM,
		KeyPEM:          keyPEM,
		TrackManifest:   manifest,
		WrapperManifest: manifest,
	})
	if err != nil {
		t.Fatalf("RunSigned: %v", err)
	}

	signedSegs := 0
	for _, tid := range w.TrackIDs() {
		for _, s := range w.Track(tid).Segments {
			data := w.SegmentData(tid, s.Seq)
			if len(data) < 8 {
				t.Fatalf("track %s segment %d too short: %d bytes", tid, s.Seq, len(data))
			}
			// A signed canonical segment leads with the c2pa uuid box.
			if got := string(data[4:8]); got != "uuid" {
				t.Errorf("track %s segment %d should start with a uuid box, got %q", tid, s.Seq, got)
			}
			signedSegs++
		}
	}
	if signedSegs == 0 {
		t.Fatal("no signed segments produced")
	}
	t.Logf("signed live HLS: %d segments", signedSegs)
}
