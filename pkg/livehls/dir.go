package livehls

import (
	"os"
	"path/filepath"
)

// WriteHLSDir writes the current live HLS artifacts to dir: a master.m3u8, a
// per-track media playlist (<trackID>.m3u8), and the per-track init segment
// (init-<trackID>.mp4, the EXT-X-MAP target). The EXT-X-BYTERANGE entries
// point at blobName — the growing fMP4 the caller is writing alongside (e.g.
// "live.fmp4"). Call after each appended segment to refresh the live
// playlists; it's cheap (a few small files).
//
// The layout is self-contained and relative, so serving `dir` over plain HTTP
// (or syncing it to a bucket) is enough for an HLS player to follow the live
// stream.
func (w *Writer) WriteHLSDir(dir, blobName string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, tid := range w.TrackIDs() {
		if init := w.InitSegment(tid); init != nil {
			if err := os.WriteFile(filepath.Join(dir, "init-"+tid+".mp4"), init, 0o644); err != nil {
				return err
			}
		}
		pl := w.MediaPlaylist(tid, "init-"+tid+".mp4", blobName)
		if err := os.WriteFile(filepath.Join(dir, tid+".m3u8"), []byte(pl), 0o644); err != nil {
			return err
		}
	}
	master := w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
	return os.WriteFile(filepath.Join(dir, "master.m3u8"), []byte(master), 0o644)
}
