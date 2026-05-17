package viewlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

var aggregateTracer = otel.Tracer("viewlog")

// RunAggregationInput plumbs everything the bootstrap-installed
// aggregator function needs to do one window pass end-to-end.
type RunAggregationInput struct {
	Store       blob.Store
	CLI         *config.CLI
	WindowStart time.Time
	WindowEnd   time.Time
	ReadMargin  time.Duration
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
		WindowStart: in.WindowStart,
		WindowEnd:   in.WindowEnd,
		ReadMargin:  in.ReadMargin,
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
	threshold := int64(result.Window.ThresholdSegments)
	for _, vc := range result.VideoCounts {
		rec := &streamplace.MediaViewCount{
			LexiconTypeID:     constants.PLACE_STREAM_MEDIA_VIEW_COUNT,
			Video:             vc.VideoURI,
			Count:             vc.Count,
			WindowStart:       in.WindowStart.UTC().Format(time.RFC3339),
			WindowEnd:         in.WindowEnd.UTC().Format(time.RFC3339),
			Methodology:       MethodologyAnySegment,
			ThresholdSegments: &threshold,
			IndexedAt:         indexedAt,
		}
		rkey := viewCountRkey(vc.VideoURI, in.WindowStart)
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
// pair, so re-runs over the same window overwrite the prior record
// instead of appending. SHA-256 hex truncated to 24 chars fits inside
// atproto's rkey character class and gives plenty of collision room.
func viewCountRkey(videoURI string, windowStart time.Time) string {
	h := sha256.New()
	h.Write([]byte(videoURI))
	h.Write([]byte{0})
	h.Write([]byte(windowStart.UTC().Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:24]
}
