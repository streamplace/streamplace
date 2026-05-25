package livehls

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteHLSDir(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	_ = w.Observe(initEvent())
	_ = w.Observe(segEvent(bytes.Repeat([]byte{1}, 100), bytes.Repeat([]byte{2}, 40)))

	dir := t.TempDir()
	if err := w.WriteHLSDir(dir, "live.fmp4"); err != nil {
		t.Fatalf("WriteHLSDir: %v", err)
	}

	for _, name := range []string{"master.m3u8", "1.m3u8", "2.m3u8", "init-1.mp4", "init-2.mp4"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to be written: %v", name, err)
		}
	}

	master, _ := os.ReadFile(filepath.Join(dir, "master.m3u8"))
	if !strings.Contains(string(master), "#EXT-X-STREAM-INF") || !strings.Contains(string(master), "1.m3u8") {
		t.Errorf("master.m3u8 missing video variant:\n%s", master)
	}

	media, _ := os.ReadFile(filepath.Join(dir, "1.m3u8"))
	for _, want := range []string{`#EXT-X-MAP:URI="init-1.mp4"`, "#EXT-X-BYTERANGE:", "live.fmp4"} {
		if !strings.Contains(string(media), want) {
			t.Errorf("1.m3u8 missing %q:\n%s", want, media)
		}
	}

	// init-1.mp4 should hold the per-track init bytes we supplied on the init event.
	if init1, _ := os.ReadFile(filepath.Join(dir, "init-1.mp4")); string(init1) != "VINIT" {
		t.Errorf("init-1.mp4 = %q, want VINIT", init1)
	}
}
