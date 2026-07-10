package viewlog

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

var aggregateTracer = otel.Tracer("viewlog")

// RunAggregationInput plumbs everything the bootstrap-installed
// aggregator function needs to do one window pass end-to-end.
type RunAggregationInput struct {
	Store          blob.Store
	CLI            *config.CLI
	WindowStart    time.Time
	WindowEnd      time.Time
	ReadMargin     time.Duration
	FetchMetafile  MetafileFetcher
	FetchTrackRefs TrackRefFetcher
}

// RunAggregation reads logs for the window, computes per-video view
// counts via AggregateWindow, and publishes one
// place.stream.media.viewCount record per video in the node's server
// repo. Idempotent across re-runs of the same window: the rkey is a
// deterministic hash of (videoURI, windowStart), so a re-run
// overwrites the existing record rather than appending.
func RunAggregation(ctx context.Context, in RunAggregationInput) error {
	ctx, span := aggregateTracer.Start(ctx, "viewlog.RunAggregation", trace.WithAttributes(
		attribute.String("window_start", in.WindowStart.Format(time.RFC3339)),
		attribute.String("window_end", in.WindowEnd.Format(time.RFC3339)),
	))
	defer span.End()

	result, err := AggregateWindow(ctx, in.Store, AggregateInput{
		WindowStart:    in.WindowStart,
		WindowEnd:      in.WindowEnd,
		ReadMargin:     in.ReadMargin,
		FetchMetafile:  in.FetchMetafile,
		FetchTrackRefs: in.FetchTrackRefs,
	})
	if err != nil {
		return fmt.Errorf("aggregate window: %w", err)
	}
	span.SetAttributes(
		attribute.Int("files_read", result.FilesRead),
		attribute.Int("events_read", result.EventsRead),
		attribute.Int("videos", len(result.VideoCounts)),
	)
	log.Log(ctx, "viewlog aggregation",
		"window_start", in.WindowStart,
		"window_end", in.WindowEnd,
		"files_read", result.FilesRead,
		"events_read", result.EventsRead,
		"videos", len(result.VideoCounts),
	)

	indexedAt := aqtime.FromTime(time.Now().UTC()).String()
	for _, vc := range result.VideoCounts {
		tracks := make([]*placestream.MediaViewCount_TrackUsage, 0, len(vc.Tracks))
		for _, t := range vc.Tracks {
			tracks = append(tracks, &placestream.MediaViewCount_TrackUsage{
				Track:      t.Track,
				Bytes:      t.Bytes,
				DurationMs: t.DurationMS,
			})
		}
		rec := &placestream.MediaViewCount{
			LexiconTypeID: constants.PLACE_STREAM_MEDIA_VIEW_COUNT,
			Video:         vc.VideoURI,
			Count:         vc.Count,
			WindowStart:   in.WindowStart.UTC().Format(time.RFC3339),
			WindowEnd:     in.WindowEnd.UTC().Format(time.RFC3339),
			Tracks:        tracks,
			IndexedAt:     indexedAt,
		}
		rkey, err := viewCountRkey(vc.VideoURI, in.WindowStart)
		if err != nil {
			log.Error(ctx, "viewlog: build viewCount rkey",
				"video", vc.VideoURI, "error", err)
			continue
		}
		if err := atproto.CommitServerRepoRecord(ctx, in.CLI, constants.PLACE_STREAM_MEDIA_VIEW_COUNT, rkey, rec); err != nil {
			// Don't bail on a single record failure — keep publishing
			// the rest and surface the error in logs. The window will
			// re-run on schedule if needed.
			log.Error(ctx, "viewlog: publish view-count record",
				"video", vc.VideoURI, "count", vc.Count, "error", err)
			continue
		}
	}
	return nil
}

// viewCountRkey derives a deterministic rkey for one (video, window)
// pair so re-runs over the same window overwrite the prior record
// instead of appending. Shape: `<windowStart-as-tid>-<video-rkey>`,
// where windowStart-as-tid is a TID with clock id 0 (every node
// agrees on this encoding) and video-rkey is the AT-URI's record key
// (itself a TID for place.stream.video). Both halves are tid-shaped
// so the joined rkey looks like atproto throughout.
func viewCountRkey(videoURI string, windowStart time.Time) (string, error) {
	aturi, err := syntax.ParseATURI(videoURI)
	if err != nil {
		return "", fmt.Errorf("parse video AT-URI %q: %w", videoURI, err)
	}
	videoTID := aturi.RecordKey().String()
	if videoTID == "" {
		return "", fmt.Errorf("video AT-URI %q has no record key", videoURI)
	}
	windowTID := syntax.NewTIDFromTime(windowStart.UTC(), 0).String()
	return windowTID + "-" + videoTID, nil
}
