package media

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"stream.place/streamplace/pkg/aqtime"
)

func (mm *MediaManager) ClipUser(ctx context.Context, user string, writer io.Writer, before *time.Time, after *time.Time) error {
	segments, err := mm.model.LatestSegmentsForUser(user, -1, before, after)
	if err != nil {
		return fmt.Errorf("unable to get segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("no segments found")
	}
	// Sort segments by StartTime, oldest first
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime.Before(segments[j].StartTime)
	})
	segmentFiles := []string{}
	for _, segment := range segments {
		aqt := aqtime.FromTime(segment.StartTime)
		fpath, err := mm.cli.SegmentFilePath(user, fmt.Sprintf("%s.%s", aqt.FileSafeString(), "mp4"))
		if err != nil {
			return fmt.Errorf("unable to get segment file path: %w", err)
		}
		segmentFiles = append(segmentFiles, fpath)
	}
	err = Clip(ctx, segmentFiles, writer)
	if err != nil {
		return fmt.Errorf("unable to clip segments: %w", err)
	}
	return nil
}
