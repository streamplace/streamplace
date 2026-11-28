package viewlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

var aggregateTracer = otel.Tracer("viewlog")

const stationPublishTimeout = 15 * time.Second

// RunAggregationInput plumbs everything the bootstrap-installed
// aggregator function needs to do one window pass end-to-end.
type RunAggregationInput struct {
	Store          blob.Store
	CLI            *config.CLI
	StationClient  vod.XRPCClient
	WindowStart    time.Time
	WindowEnd      time.Time
	ReadMargin     time.Duration
	FetchMetafile  MetafileFetcher
	FetchTrackRefs TrackRefFetcher
}

// RunAggregation publishes one view count per video and window, in the
// configured station's PDS or, in standalone mode, the node's server repo.
func RunAggregation(ctx context.Context, in RunAggregationInput) error {
	ctx, span := aggregateTracer.Start(ctx, "viewlog.RunAggregation", trace.WithAttributes(
		attribute.String("window_start", in.WindowStart.Format(time.RFC3339)),
		attribute.String("window_end", in.WindowEnd.Format(time.RFC3339)),
	))
	defer span.End()

	var did, head string
	if in.CLI != nil {
		did = in.CLI.StationDID
	}
	if did != "" {
		if !in.CLI.StationOriginPublisher {
			return errors.New("station view-count aggregation requires --station-origin-publisher")
		}
		if in.StationClient == nil {
			return errors.New("station view-count aggregation requires an authenticated station PDS client")
		}
		// Fence the input snapshot, not just the write. A concurrent late-data
		// job can otherwise publish newer counts while this job reads logs.
		headCtx, cancel := context.WithTimeout(ctx, stationPublishTimeout)
		var latest comatproto.SyncGetLatestCommit_Output
		err := in.StationClient.Do(headCtx, xrpc.Query, "", "com.atproto.sync.getLatestCommit",
			map[string]any{"did": did}, nil, &latest)
		cancel()
		if err != nil {
			return errors.New("could not read station PDS commit for view-count aggregation")
		}
		if _, err := cid.Decode(latest.Cid); err != nil {
			return errors.New("station PDS returned an invalid commit CID")
		}
		head = latest.Cid
	}

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
	var publishErr error
	for _, vc := range result.VideoCounts {
		tracks := make([]placestream.MediaViewCount_TrackUsage, 0, len(vc.Tracks))
		for _, t := range vc.Tracks {
			tracks = append(tracks, placestream.MediaViewCount_TrackUsage{
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
		if did != "" {
			next, err := putStationViewCount(ctx, in.StationClient, did, head, in.WindowStart, rec)
			if err != nil {
				return err // Retry from a fresh head and input snapshot, never this batch.
			}
			head = next
			continue
		}
		rkey, err := viewCountRkey(vc.VideoURI, in.WindowStart)
		if err != nil {
			log.Error(ctx, "viewlog: build viewCount rkey",
				"video", vc.VideoURI, "error", err)
			publishErr = errors.Join(publishErr, err)
			continue
		}
		if err := statedb.CheckTaskClaim(ctx); err != nil {
			return err
		}
		if err := atproto.CommitServerRepoRecord(ctx, in.CLI, constants.PLACE_STREAM_MEDIA_VIEW_COUNT, rkey, rec); err != nil {
			// Other records are independent; retry the window using the same keys.
			log.Error(ctx, "viewlog: publish view-count record",
				"video", vc.VideoURI, "count", vc.Count, "error", err)
			publishErr = errors.Join(publishErr, err)
			continue
		}
	}
	return publishErr
}

func putStationViewCount(ctx context.Context, client vod.XRPCClient, did, head string, windowStart time.Time, rec *placestream.MediaViewCount) (string, error) {
	rkey, err := stationViewCountRkey(rec.Video, windowStart)
	if err != nil {
		return "", err
	}
	body := &comatproto.RepoPutRecord_Input{
		Repo:       did,
		Collection: constants.PLACE_STREAM_MEDIA_VIEW_COUNT,
		Rkey:       rkey,
		Record:     &glex.LexiconTypeDecoder{Val: rec},
		SwapCommit: &head,
	}
	var out comatproto.RepoPutRecord_Output
	writeCtx, cancel := context.WithTimeout(ctx, stationPublishTimeout)
	defer cancel()
	if err := statedb.CheckTaskClaim(ctx); err != nil {
		return "", err
	}
	if err := client.Do(writeCtx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", nil, body, &out); err != nil {
		var rpcErr *xrpc.XRPCError
		if errors.As(err, &rpcErr) && rpcErr.ErrStr == "InvalidSwap" {
			return "", errors.New("station PDS commit changed during view-count aggregation")
		}
		return "", errors.New("station PDS did not acknowledge view-count putRecord")
	}
	if out.Uri != "at://"+did+"/"+body.Collection+"/"+rkey || out.Commit == nil {
		return "", errors.New("station PDS returned an invalid view-count write acknowledgement")
	}
	if _, err := cid.Decode(out.Cid); err != nil {
		return "", errors.New("station PDS returned an invalid view-count record CID")
	}
	if _, err := cid.Decode(out.Commit.Cid); err != nil {
		return "", errors.New("station PDS returned an invalid view-count commit CID")
	}
	return out.Commit.Cid, nil
}

func stationViewCountRkey(videoURI string, windowStart time.Time) (string, error) {
	uri, err := syntax.ParseATURI(videoURI)
	if err != nil || uri.Collection().String() != constants.PLACE_STREAM_VIDEO || uri.RecordKey().String() == "" {
		return "", fmt.Errorf("invalid video record AT-URI %q", videoURI)
	}
	digest := sha256.Sum256([]byte(uri.String()))
	return syntax.NewTIDFromTime(windowStart.UTC(), 0).String() + "-" + hex.EncodeToString(digest[:]), nil
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
