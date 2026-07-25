package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
)

const moderationRetention = 120 * time.Second

func StartSegmentCleaner(ctx context.Context, localDB localdb.LocalDB, cli *config.CLI) error {
	ctx = log.WithLogValues(ctx, "func", "StartSegmentCleaner")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(60 * time.Second):
				expiredSegments, err := localDB.GetExpiredSegments(ctx)
				if err != nil {
					return err
				}
				log.Log(ctx, "Cleaning expired segments", "count", len(expiredSegments))
				for _, seg := range expiredSegments {
					g.Go(func() error {
						err := deleteSegment(ctx, localDB, cli, seg)
						if err != nil {
							log.Error(ctx, "Failed to delete segment", "error", err)
						}
						return nil
					})

				}
			}
		}
	})

	return g.Wait()
}

func deleteSegment(ctx context.Context, localDB localdb.LocalDB, cli *config.CLI, seg localdb.Segment) error {
	if time.Since(seg.StartTime) < moderationRetention {
		log.Debug(ctx, "Skipping deletion of segment for moderation retention", "id", seg.ID, "time since start", time.Since(seg.StartTime))
		return nil
	}
	aqt := aqtime.FromTime(seg.StartTime)
	// Segments are archived as .m4s (see distributeSegment); this used to say
	// "mp4" (a leftover from presentation-MP4 days), so os.Remove always missed
	// the real file and the .m4s bytes leaked on disk forever.
	fpath, err := cli.SegmentFilePath(seg.RepoDID, fmt.Sprintf("%s.%s", aqt.FileSafeString(), "m4s"))
	if err != nil {
		return err
	}
	err = os.Remove(fpath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	err = localDB.DeleteSegment(ctx, seg.ID)
	if err != nil {
		return err
	}
	return nil
}
