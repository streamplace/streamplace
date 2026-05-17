package viewlog

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
)

// MethodologyAnySegment is the initial counting heuristic: a distinct
// sid that fetched at least `ThresholdSegments` segments for a video
// inside the aggregation window. Recorded on every published view-
// count record so consumers can interpret the number against the
// algorithm that produced it; later methodologies (ms-from-metafile,
// client-reported, …) ship under their own tags.
const MethodologyAnySegment = "any-segment"

// AggregateInput bundles the window + tunables for one aggregation
// pass. WindowStart is inclusive, WindowEnd is exclusive — matches the
// AT-URI record's wire shape.
type AggregateInput struct {
	WindowStart       time.Time
	WindowEnd         time.Time
	ThresholdSegments int
	// ReadMargin extends the file-listing window backwards so a file
	// opened slightly before WindowStart (whose events may straddle
	// the boundary) is still read. Per-event filtering by ev.Ts keeps
	// the count correct; this just decides which files to crack open.
	// Defaults to 1 hour, which is generously larger than the writer's
	// flush interval.
	ReadMargin time.Duration
}

// VideoCount is one row in the AggregateResult: distinct qualifying
// sids per place.stream.video over the window.
type VideoCount struct {
	VideoURI string
	Count    int64
}

// AggregateResult is the return shape of AggregateWindow. The caller
// turns each VideoCount into a place.stream.media.viewCount record.
type AggregateResult struct {
	Window      AggregateInput
	VideoCounts []VideoCount
	// FilesRead and EventsRead surface aggregation effort for ops
	// observability; not load-bearing for the records themselves.
	FilesRead  int
	EventsRead int
}

// AggregateWindow lists every view-log file under view-logs/ whose
// filename timestamp falls in [WindowStart, WindowEnd), reads each as
// gzipped JSONL, and counts distinct (sid, video) pairs that crossed
// the segment threshold. Manifest events provide the sid → video URI
// mapping segment events inherit from; segment_requests for a sid we
// never saw a manifest_request for are unattributed and dropped.
func AggregateWindow(ctx context.Context, store blob.Store, in AggregateInput) (*AggregateResult, error) {
	if in.WindowEnd.Before(in.WindowStart) || in.WindowEnd.Equal(in.WindowStart) {
		return nil, fmt.Errorf("viewlog: AggregateWindow needs a positive window, got start=%s end=%s",
			in.WindowStart, in.WindowEnd)
	}
	threshold := in.ThresholdSegments
	if threshold <= 0 {
		threshold = 1
	}
	readMargin := in.ReadMargin
	if readMargin <= 0 {
		readMargin = time.Hour
	}

	keys, err := store.List(ctx, viewLogsPrefix)
	if err != nil {
		return nil, fmt.Errorf("viewlog: list logs: %w", err)
	}

	// Pick files whose openedAt timestamp falls in
	// [WindowStart - ReadMargin, WindowEnd). Events inside each file
	// are filtered to the exact window below — the read margin just
	// makes sure we don't skip a file whose contents straddle the
	// boundary.
	readStart := in.WindowStart.Add(-readMargin)
	type pickedFile struct {
		key string
		ts  time.Time
	}
	var picked []pickedFile
	for _, k := range keys {
		ts, ok := parseViewLogKeyTime(k)
		if !ok {
			continue
		}
		if ts.Before(readStart) || !ts.Before(in.WindowEnd) {
			continue
		}
		picked = append(picked, pickedFile{key: k, ts: ts})
	}
	// Time-order the read so per-sid manifest_request events arrive
	// before their segment_requests, regardless of which node wrote
	// them. Pure-lex sort on the key would interleave by node-DID
	// before timestamp; we want timestamp first.
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].ts.Equal(picked[j].ts) {
			return picked[i].key < picked[j].key
		}
		return picked[i].ts.Before(picked[j].ts)
	})

	// sidVideo holds the most recent video URI a sid asked about. A
	// segment_request inherits this; if the sid is unknown, the event
	// is dropped (unattributed).
	sidVideo := make(map[string]string)
	// segments counts segment_requests per (sid, video) pair.
	type pair struct {
		sid   string
		video string
	}
	segments := make(map[pair]int)

	var eventsRead int
	for _, pf := range picked {
		if err := readJSONLGz(ctx, store, pf.key, func(ev *Event) {
			// Manifest events flow into the sid→video map even when
			// they're outside the window — a session might have
			// started before WindowStart but kept fetching segments
			// inside it. Only segment_request events are clipped to
			// [WindowStart, WindowEnd).
			switch ev.Type {
			case EventTypeManifestRequest:
				if ev.SID != "" && ev.VideoURI != "" {
					sidVideo[ev.SID] = ev.VideoURI
				}
				eventsRead++
			case EventTypeSegmentRequest:
				if ev.Ts.Before(in.WindowStart) || !ev.Ts.Before(in.WindowEnd) {
					return
				}
				if ev.SID == "" {
					return
				}
				video := sidVideo[ev.SID]
				if video == "" {
					return
				}
				segments[pair{sid: ev.SID, video: video}]++
				eventsRead++
			}
		}); err != nil {
			// Per-file read failures are logged + skipped rather than
			// aborting the entire window. One corrupt file shouldn't
			// drop every video's count.
			log.Error(ctx, "viewlog: read log file", "key", pf.key, "error", err)
			continue
		}
	}

	// Tally distinct qualifying sids per video.
	perVideo := make(map[string]int64)
	for p, n := range segments {
		if n >= threshold {
			perVideo[p.video]++
		}
	}

	out := make([]VideoCount, 0, len(perVideo))
	for v, c := range perVideo {
		out = append(out, VideoCount{VideoURI: v, Count: c})
	}
	// Sort for deterministic output (mostly for tests + record-key
	// stability when callers iterate in slice order).
	sort.Slice(out, func(i, j int) bool { return out[i].VideoURI < out[j].VideoURI })

	return &AggregateResult{
		Window:      AggregateInput{WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, ThresholdSegments: threshold},
		VideoCounts: out,
		FilesRead:   len(picked),
		EventsRead:  eventsRead,
	}, nil
}

// viewLogsPrefix is the top of the per-node log tree, shared by every
// writer that targets the same store.
const viewLogsPrefix = "view-logs/"

// keyTimeFormat matches the suffix the Writer attaches to each rotated
// blob. RFC3339-ish but with ':' replaced by '-' so the key stays a
// valid filename on every platform. Nanosecond precision keeps
// closely-spaced flushes (size-triggered storms during a popular video's
// hot moment) from sharing a key and overwriting each other.
const keyTimeFormat = "2006-01-02T15-04-05.000000000Z"

// parseViewLogKeyTime extracts the rotated-at timestamp from a key
// shaped like `view-logs/<node-did>/<window>.jsonl.gz`. Returns
// (zero, false) for anything that doesn't fit the shape.
func parseViewLogKeyTime(key string) (time.Time, bool) {
	base := path.Base(key)
	const suffix = ".jsonl.gz"
	if !strings.HasSuffix(base, suffix) {
		return time.Time{}, false
	}
	stamp := strings.TrimSuffix(base, suffix)
	t, err := time.Parse(keyTimeFormat, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// readJSONLGz opens key, gunzips, and calls fn for each decoded Event.
// JSON parse failures on a single line are logged + skipped rather
// than aborting the whole file — a malformed line shouldn't lose
// every event that follows it.
func readJSONLGz(ctx context.Context, store blob.Store, key string, fn func(*Event)) error {
	r, err := store.Open(ctx, key)
	if err != nil {
		return fmt.Errorf("open %s: %w", key, err)
	}
	defer r.Close()
	gz, err := gzip.NewReader(io.NewSectionReader(r, 0, r.Size()))
	if err != nil {
		return fmt.Errorf("gunzip %s: %w", key, err)
	}
	defer gz.Close()
	sc := bufio.NewScanner(gz)
	// JSONL lines can be larger than the default 64KB scanner buffer.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			log.Debug(ctx, "viewlog: skipping malformed line", "key", key, "error", err)
			continue
		}
		fn(&ev)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("scan %s: %w", key, err)
	}
	return nil
}
