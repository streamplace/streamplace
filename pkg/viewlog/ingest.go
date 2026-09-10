package viewlog

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/cdn"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// IngestCursor remembers which CDN log parts have already been turned
// into view-log files, so every node in a cluster can run the ingester
// and each part is still processed exactly once. statedb implements
// it; tests use an in-memory fake.
type IngestCursor interface {
	// ClaimPart marks (source, partID) in-progress for this caller.
	// Returns false when the part is already complete, or claimed by
	// someone else less than staleAfter ago (a crashed ingester's
	// claim goes stale and is retaken).
	ClaimPart(ctx context.Context, source, partID string, staleAfter time.Duration) (bool, error)
	// CompletePart records a successful ingest.
	CompletePart(ctx context.Context, source, partID string, events int) error
	// ReleasePart drops an in-progress claim after a failure so the
	// next run retries immediately instead of waiting out staleAfter.
	ReleasePart(ctx context.Context, source, partID string) error
}

// IngestInput bundles one ingest pass over a CDN's archived logs.
type IngestInput struct {
	// Store is the VOD blob store the view-log files are written into
	// (the same one the node's own Writer targets).
	Store blob.Store
	// Source lists + decodes the provider's archived logs.
	Source cdn.LogSource
	// SourceName identifies the provider in the output key layout,
	// `view-logs/cdn-<SourceName>/...`, and in the cursor.
	SourceName string
	Salts      *SaltManager
	Cursor     IngestCursor
	// Window is the aggregation bucket size. Each part is re-bucketed
	// into one output file per window so the aggregator's filename-
	// timestamp file selection finds it without a huge read margin.
	Window time.Duration
	// Lookback bounds how far back parts are listed. Parts older than
	// this are never considered, even if unseen. Defaults to 48h,
	// comfortably past bunny's "the next day" delivery.
	Lookback time.Duration
	// StaleAfter is how long a claim on a part stands before another
	// ingester may retake it. Defaults to 1h.
	StaleAfter time.Duration
	// Reaggregate is called once per (part, window) after the part's
	// files are all visible, so the already-published view counts for
	// that window get recomputed with the CDN's segments included.
	// Optional.
	Reaggregate func(ctx context.Context, partID string, start, end time.Time) error
	// Now overrides the clock (tests).
	Now func() time.Time
}

// IngestResult summarises one pass for logs + tests.
type IngestResult struct {
	PartsListed   int
	PartsIngested int
	// Events is the count of segment_request events written; Skipped
	// is lines that were not blob fetches (non-2xx, other paths, no
	// sid) and were dropped.
	Events  int
	Skipped int
	Windows int
}

// RunIngest lists the source's parts within Lookback, claims each
// unseen one, streams it into per-window view-log files, marks it
// complete, and asks for the touched windows to be re-aggregated. A
// failure on one part releases its claim and moves on; the first
// error is returned after every part has been attempted.
func RunIngest(ctx context.Context, in IngestInput) (*IngestResult, error) {
	if in.Store == nil || in.Source == nil || in.Cursor == nil || in.Salts == nil {
		return nil, errors.New("viewlog: RunIngest needs Store, Source, Cursor and Salts")
	}
	if in.SourceName == "" {
		return nil, errors.New("viewlog: RunIngest needs a SourceName")
	}
	if in.Window <= 0 {
		return nil, errors.New("viewlog: RunIngest needs a positive Window")
	}
	now := time.Now
	if in.Now != nil {
		now = in.Now
	}
	lookback := in.Lookback
	if lookback <= 0 {
		lookback = 48 * time.Hour
	}
	staleAfter := in.StaleAfter
	if staleAfter <= 0 {
		staleAfter = time.Hour
	}

	parts, err := in.Source.ListParts(ctx, now().Add(-lookback))
	if err != nil {
		return nil, fmt.Errorf("viewlog: list cdn log parts: %w", err)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].ID < parts[j].ID })
	res := &IngestResult{PartsListed: len(parts)}
	var firstErr error
	for _, part := range parts {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		claimed, err := in.Cursor.ClaimPart(ctx, in.SourceName, part.ID, staleAfter)
		if err != nil {
			return res, fmt.Errorf("viewlog: claim part %s: %w", part.ID, err)
		}
		if !claimed {
			continue
		}
		pr, err := ingestPart(ctx, in, part)
		if err != nil {
			log.Error(ctx, "viewlog: ingest cdn log part failed", "source", in.SourceName, "part", part.ID, "error", err)
			if rerr := in.Cursor.ReleasePart(ctx, in.SourceName, part.ID); rerr != nil {
				log.Error(ctx, "viewlog: release part claim", "part", part.ID, "error", rerr)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// Request re-aggregation BEFORE marking the part complete. A
		// completed part is never revisited, so an enqueue that fails
		// after completion would leave that window's published count
		// permanently missing the CDN's segments. Failing here instead
		// releases the claim; the next pass re-ingests the part, which
		// is idempotent (output keys derive from part ID + window, so
		// the rewrite lands on the same files with the same content).
		if err := requestReaggregation(ctx, in, part.ID, pr.windows); err != nil {
			log.Error(ctx, "viewlog: request re-aggregation failed", "source", in.SourceName, "part", part.ID, "error", err)
			if rerr := in.Cursor.ReleasePart(ctx, in.SourceName, part.ID); rerr != nil {
				log.Error(ctx, "viewlog: release part claim", "part", part.ID, "error", rerr)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := in.Cursor.CompletePart(ctx, in.SourceName, part.ID, pr.events); err != nil {
			// Files + re-aggregation requests are in place; the next
			// pass will redo both harmlessly. Surface it anyway.
			return res, fmt.Errorf("viewlog: complete part %s: %w", part.ID, err)
		}
		res.PartsIngested++
		res.Events += pr.events
		res.Skipped += pr.skipped
		res.Windows += len(pr.windows)
		log.Log(ctx, "viewlog: ingested cdn log part",
			"source", in.SourceName, "part", part.ID, "size", part.Size,
			"events", pr.events, "skipped", pr.skipped, "windows", len(pr.windows))
	}
	return res, firstErr
}

// requestReaggregation asks for every touched window to be recomputed.
// Stops at the first failure: the caller releases the part and the
// whole thing is retried next pass (duplicate requests dedup on their
// task key).
func requestReaggregation(ctx context.Context, in IngestInput, partID string, windows []time.Time) error {
	if in.Reaggregate == nil {
		return nil
	}
	for _, w := range windows {
		if err := in.Reaggregate(ctx, partID, w, w.Add(in.Window)); err != nil {
			return fmt.Errorf("window %s: %w", w.Format(time.RFC3339), err)
		}
	}
	return nil
}

type partResult struct {
	events, skipped int
	windows         []time.Time
}

// bucketWriter is one open per-window output file.
type bucketWriter struct {
	w  blob.Writer
	gz *gzip.Writer
	n  int
}

// ingestPart streams one part into per-window files. Nothing becomes
// visible until every window's file is complete, so a mid-part
// failure leaves no partial data behind for the aggregator to count.
func ingestPart(ctx context.Context, in IngestInput, part cdn.Part) (*partResult, error) {
	prefix := viewLogsPrefix + "cdn-" + in.SourceName + "/" + sanitizePartID(part.ID) + "/"
	buckets := make(map[time.Time]*bucketWriter)
	abortAll := func() {
		for _, b := range buckets {
			_ = b.w.Close()
		}
	}
	pr := &partResult{}
	err := in.Source.ReadPart(ctx, part, func(r cdn.Request) error {
		ev, ok := segmentEventFromRequest(r, in.Salts)
		if !ok {
			pr.skipped++
			return nil
		}
		start := ev.Ts.Truncate(in.Window)
		b := buckets[start]
		if b == nil {
			w, err := in.Store.NewWriter(ctx, prefix+start.UTC().Format(keyTimeFormat)+".jsonl.gz", "application/gzip")
			if err != nil {
				return fmt.Errorf("open window file: %w", err)
			}
			b = &bucketWriter{w: w, gz: gzip.NewWriter(w)}
			buckets[start] = b
		}
		line, err := json.Marshal(&ev)
		if err != nil {
			return err
		}
		if _, err := b.gz.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write window file: %w", err)
		}
		b.n++
		pr.events++
		return nil
	})
	if err != nil {
		abortAll()
		return nil, err
	}
	for start, b := range buckets {
		if err := b.gz.Close(); err != nil {
			abortAll()
			return nil, fmt.Errorf("gzip close: %w", err)
		}
		if err := b.w.Complete(); err != nil {
			abortAll()
			return nil, fmt.Errorf("complete window file: %w", err)
		}
		pr.windows = append(pr.windows, start)
	}
	sort.Slice(pr.windows, func(i, j int) bool { return pr.windows[i].Before(pr.windows[j]) })
	return pr, nil
}

// sanitizePartID flattens a provider part ID (usually a storage path)
// into one key segment.
func sanitizePartID(id string) string {
	return strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(id)
}

// segmentEventFromRequest maps a CDN access-log record onto the same
// segment_request event the node's own blob handler logs. Only
// successful fetches of blobs/<cid>.mp4 that carry a playback sid
// qualify; everything else (playlists don't go through the CDN,
// probes, 403s from expired tokens, sid-less transfers) is dropped.
func segmentEventFromRequest(r cdn.Request, salts *SaltManager) (Event, bool) {
	if r.Status != http.StatusOK && r.Status != http.StatusPartialContent {
		return Event{}, false
	}
	u, err := url.Parse(r.URL)
	if err != nil {
		return Event{}, false
	}
	base := path.Base(u.Path)
	if !strings.HasSuffix(base, ".mp4") || path.Base(path.Dir(u.Path)) != strings.TrimSuffix(vod.BlobsPrefix, "/") {
		return Event{}, false
	}
	cid := strings.TrimSuffix(base, ".mp4")
	q := u.Query()
	sid := q.Get("sid")
	if cid == "" || sid == "" {
		return Event{}, false
	}
	ts := r.Time.UTC()
	ipHash, err := salts.HashIP(r.RemoteIP, ts)
	if err != nil {
		// Same policy as the live handlers: a missing hash is better
		// than a missing view.
		ipHash = ""
	}
	return Event{
		Ts:        ts,
		Type:      EventTypeSegmentRequest,
		SID:       sid,
		IPHash:    ipHash,
		CID:       cid,
		OwnerDID:  q.Get("did"),
		BytesSent: r.BytesSent,
	}, true
}

// ReaggregateTaskKey is the dedup key for re-running one aggregation
// window because CDN part `partID` added segments to it. Distinct
// from AggregateTaskKey so the scheduled first pass (already done by
// the time CDN logs arrive) doesn't swallow the re-run.
func ReaggregateTaskKey(start, end time.Time, partID string) string {
	return AggregateTaskKey(start, end) + "::reagg::" + sanitizePartID(partID)
}

// IngestTaskKey is the dedup key for one scheduled ingest tick, so a
// cluster runs one ingest per interval rather than one per node.
func IngestTaskKey(tick time.Time) string {
	return "cdn-log-ingest::" + tick.UTC().Format(time.RFC3339)
}

// ScheduleIngest runs until ctx is cancelled, enqueueing one
// TaskCDNLogIngest per Interval, keyed on the UTC-aligned tick so
// every node's attempt dedups to a single task.
func ScheduleIngest(ctx context.Context, q TaskEnqueuer, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("viewlog: ScheduleIngest needs a positive interval")
	}
	tryEnqueueIngest(ctx, q, interval, time.Now().UTC())
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-tick.C:
			tryEnqueueIngest(ctx, q, interval, now.UTC())
		}
	}
}

func tryEnqueueIngest(ctx context.Context, q TaskEnqueuer, interval time.Duration, now time.Time) {
	tick := now.Truncate(interval)
	payload := statedb.CDNLogIngestTask{Tick: tick}
	if _, err := q.EnqueueTask(ctx, statedb.TaskCDNLogIngest, payload, statedb.WithTaskKey(IngestTaskKey(tick))); err != nil {
		log.Error(ctx, "viewlog: enqueue cdn log ingest task", "tick", tick, "error", err)
	}
}
