package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/muxl"
)

// ClipUser concatenates a user's stored canonical .m4s segments over the given
// time window into a single flat MP4 written to writer (used for
// moderation/report clips). The segments are blindly concatenatable MUXL, so
// the clip is just their bytes wrapped in a synthesized ftyp+moov — no remux,
// and each segment's C2PA signature passes through verbatim.
func ClipUser(ctx context.Context, localDB localdb.LocalDB, cli *config.CLI, user string, writer io.Writer, before *time.Time, after *time.Time) error {
	segments, err := localDB.LatestSegmentsForUser(user, -1, false, before, after)
	if err != nil {
		return fmt.Errorf("unable to get segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("no segments found")
	}
	// Oldest first, so the clip plays in order.
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime.Before(segments[j].StartTime)
	})
	readers := make([]io.Reader, 0, len(segments))
	for _, segment := range segments {
		aqt := aqtime.FromTime(segment.StartTime)
		fpath, err := cli.SegmentFilePath(user, fmt.Sprintf("%s.%s", aqt.FileSafeString(), "m4s"))
		if err != nil {
			return fmt.Errorf("unable to get segment file path: %w", err)
		}
		fd, err := os.Open(fpath)
		if err != nil {
			return fmt.Errorf("unable to open segment file: %w", err)
		}
		defer fd.Close()
		readers = append(readers, fd)
	}
	// muxl wrap unwraps every concatenated segment, aggregates their catalogs,
	// and synthesizes one flat MP4 over the lot.
	if err := muxl.RunMuxlWrap(ctx, io.MultiReader(readers...), "flat", writer); err != nil {
		return fmt.Errorf("unable to clip segments: %w", err)
	}
	return nil
}
