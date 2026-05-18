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

	comatproto "github.com/bluesky-social/indigo/api/atproto"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/vod"
)

// MetafileFetcher resolves a blob CID to its parsed metafile. The
// aggregator calls it once per unique CID seen in the window's
// segment_request events; tests can substitute a fake.
type MetafileFetcher func(ctx context.Context, cid string) (*vod.Metafile, error)

// TrackRefFetcher resolves a blob CID to the strongRefs of the
// place.stream.media.track records whose muxlTrack lives in that
// blob, keyed by the muxlTrack's in-container trackId. The aggregator
// looks each in-container tid up to attribute bytes/duration to the
// owning track record — user-contributed tracks (transcripts,
// transcodes) attribute to their own record, the original tracks
// attribute to the streamer's record.
//
// Returning a nil map (no error) is fine; the corresponding bytes
// just don't get a per-track row in the output.
type TrackRefFetcher func(ctx context.Context, cid string) (map[string]*comatproto.RepoStrongRef, error)

// AggregateInput bundles the window + tunables for one aggregation
// pass. WindowStart is inclusive, WindowEnd is exclusive — matches the
// AT-URI record's wire shape.
type AggregateInput struct {
	WindowStart time.Time
	WindowEnd   time.Time
	// ThresholdSegments is the floor on segment_requests per (sid,
	// video) before that sid counts as a view. Defaults to 1.
	// Operator-level knob, not surfaced on the published record.
	ThresholdSegments int
	// ReadMargin extends the file-listing window backwards so a file
	// opened slightly before WindowStart (whose events may straddle
	// the boundary) is still read. Per-event filtering by ev.Ts keeps
	// the count correct; this just decides which files to crack open.
	// Defaults to 1 hour, which is generously larger than the writer's
	// flush interval.
	ReadMargin time.Duration
	// FetchMetafile resolves a CID to its parsed metafile. Defaults to
	// reading blobs/<cid>.json from the same store the logs live in.
	// A nil return + nil error means "metafile not found" and the
	// segment_request is counted toward the sid view but contributes
	// zero bytes / duration.
	FetchMetafile MetafileFetcher
	// FetchTrackRefs resolves a CID to the strongRefs of the
	// place.stream.media.track records whose muxlTrack lives in that
	// blob, keyed by in-container trackId. Required for per-track
	// usage rows; when nil or returning no entry for a given tid the
	// bytes/duration credit for that track is dropped (the sid view
	// still counts).
	FetchTrackRefs TrackRefFetcher
}

// VideoCount is one row in the AggregateResult: distinct qualifying
// sids per place.stream.video over the window plus per-track totals
// of bytes + duration transferred.
type VideoCount struct {
	VideoURI string
	Count    int64
	Tracks   []TrackUsage
}

// TrackUsage is the objective half of the aggregator's output: how
// many bytes + how much playback duration were served from one
// place.stream.media.track record inside the window. The strongRef
// points at the track record whose bytes were transferred.
type TrackUsage struct {
	Track      *comatproto.RepoStrongRef
	Bytes      int64
	DurationMS int64
}

// AggregateResult is the return shape of AggregateWindow. The caller
// turns each VideoCount into a place.stream.media.viewCount record.
type AggregateResult struct {
	Window      AggregateInput
	VideoCounts []VideoCount
	// FilesRead and EventsRead surface aggregation effort for ops
	// observability; not load-bearing for the records themselves.
	FilesRead       int
	EventsRead      int
	MetafilesLoaded int
}

// AggregateWindow lists every view-log file under view-logs/ whose
// filename timestamp falls in [WindowStart - ReadMargin, WindowEnd),
// reads each as gzipped JSONL, and produces per-video totals:
//
//   - Count: distinct sids that hit ≥ ThresholdSegments segment_requests
//   - Tracks: per-track bytes + duration credited by intersecting each
//     segment_request's HTTP Range with the metafile's byte layout
//
// Manifest events provide the sid → video URI mapping segment events
// inherit from; segment_requests for a sid we never saw a manifest
// for are unattributed and dropped (no video to credit).
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
	fetchMetafile := in.FetchMetafile
	if fetchMetafile == nil {
		fetchMetafile = func(ctx context.Context, cid string) (*vod.Metafile, error) {
			return fetchMetafileFromStore(ctx, store, cid)
		}
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
	// trackTotals accumulates objective bytes + duration per
	// (video, track-record). Keyed on (video, trackURI) since the
	// strongRef's URI is globally unique — same CID across two
	// videos (parent + clip) still attributes to the right tracks.
	type trackKey struct {
		video    string
		trackURI string
	}
	trackTotals := make(map[trackKey]*TrackUsage)
	// metafileCache + trackRefCache: one fetch per CID across the
	// whole window. The nil values are cached too, so a missing
	// metafile / refs lookup doesn't trigger repeated fetches.
	metafileCache := make(map[string]*vod.Metafile)
	trackRefCache := make(map[string]map[string]*comatproto.RepoStrongRef)
	metafilesLoaded := 0

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

				// Per-track byte/duration accounting needs (a) the
				// metafile that maps blob offsets to per-track
				// segments and (b) the strongRef for each track's
				// place.stream.media.track record. Both cached per
				// CID; nil cache entries mean "tried + missing" so
				// we don't refetch.
				if ev.CID == "" || ev.RangeEnd < ev.RangeStart {
					return
				}
				meta, cached := metafileCache[ev.CID]
				if !cached {
					m, err := fetchMetafile(ctx, ev.CID)
					if err != nil {
						log.Debug(ctx, "viewlog: fetch metafile",
							"cid", ev.CID, "error", err)
					}
					meta = m
					metafileCache[ev.CID] = meta
					if meta != nil {
						metafilesLoaded++
					}
				}
				if meta == nil {
					return
				}
				refs, refsCached := trackRefCache[ev.CID]
				if !refsCached {
					if in.FetchTrackRefs != nil {
						r, err := in.FetchTrackRefs(ctx, ev.CID)
						if err != nil {
							log.Debug(ctx, "viewlog: fetch track refs",
								"cid", ev.CID, "error", err)
						}
						refs = r
					}
					trackRefCache[ev.CID] = refs
				}
				for tid, track := range meta.Tracks {
					bytes, durMS := rangeOverlapInTrack(ev.RangeStart, ev.RangeEnd, track)
					if bytes == 0 {
						continue
					}
					ref := refs[tid]
					if ref == nil {
						// No track record found for this in-container
						// tid. Drop the credit — a TrackUsage row
						// without a stable strongRef wouldn't be
						// useful to consumers.
						continue
					}
					k := trackKey{video: video, trackURI: ref.Uri}
					tu, ok := trackTotals[k]
					if !ok {
						tu = &TrackUsage{Track: ref}
						trackTotals[k] = tu
					}
					tu.Bytes += bytes
					tu.DurationMS += durMS
				}
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

	// Group tracks under each video. A video can show up here with
	// zero qualifying sids (bytes were transferred but every sid fell
	// below threshold) — emit it anyway so the objective totals
	// aren't lost.
	videoSet := make(map[string]struct{})
	for v := range perVideo {
		videoSet[v] = struct{}{}
	}
	tracksByVideo := make(map[string][]TrackUsage)
	for k, tu := range trackTotals {
		tracksByVideo[k.video] = append(tracksByVideo[k.video], *tu)
		videoSet[k.video] = struct{}{}
	}

	out := make([]VideoCount, 0, len(videoSet))
	for v := range videoSet {
		tracks := tracksByVideo[v]
		// Stable per-video track order so re-runs produce byte-
		// identical records. Sort on the strongRef URI (globally
		// unique) since that's the only stable identity now that
		// in-container tids aren't surfaced.
		sort.Slice(tracks, func(i, j int) bool {
			return tracks[i].Track.Uri < tracks[j].Track.Uri
		})
		out = append(out, VideoCount{
			VideoURI: v,
			Count:    perVideo[v],
			Tracks:   tracks,
		})
	}
	// Sort for deterministic output (mostly for tests + record-key
	// stability when callers iterate in slice order).
	sort.Slice(out, func(i, j int) bool { return out[i].VideoURI < out[j].VideoURI })

	return &AggregateResult{
		Window:          AggregateInput{WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, ThresholdSegments: threshold, ReadMargin: readMargin},
		VideoCounts:     out,
		FilesRead:       len(picked),
		EventsRead:      eventsRead,
		MetafilesLoaded: metafilesLoaded,
	}, nil
}

// rangeOverlapInTrack returns the (bytes, durationMS) credit one
// HTTP Range request earns against a single muxlTrack's byte layout.
//
// HLS playback typically issues one byte-range request per
// EXT-X-BYTERANGE entry, so the request usually matches one
// MetafileSegment exactly. Partial overlaps (chunked range fetches,
// resumed downloads) credit a proportional share of duration so the
// numbers stay roughly honest under either pattern.
//
// rangeStart and rangeEnd are inclusive (RFC 7233). track.Segments'
// Offset is the absolute byte offset within the blob; Size is the
// segment's byte length; DurationTicks is the segment's playback
// length in track timescale units.
func rangeOverlapInTrack(rangeStart, rangeEnd int64, track vod.MetafileTrack) (bytes, durationMS int64) {
	if rangeEnd < rangeStart || track.Timescale == 0 {
		return 0, 0
	}
	for _, seg := range track.Segments {
		if seg.Size <= 0 {
			continue
		}
		segLast := seg.Offset + seg.Size - 1
		overlapStart := rangeStart
		if seg.Offset > overlapStart {
			overlapStart = seg.Offset
		}
		overlapEnd := rangeEnd
		if segLast < overlapEnd {
			overlapEnd = segLast
		}
		if overlapEnd < overlapStart {
			continue
		}
		overlap := overlapEnd - overlapStart + 1
		bytes += overlap
		// Byte-proportional duration: (overlap / size) * segment-ms.
		// Avoid float math so deterministic runs produce identical
		// records.
		segDurMS := int64(seg.DurationTicks) * 1000 / int64(track.Timescale)
		durationMS += segDurMS * overlap / seg.Size
	}
	return bytes, durationMS
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

// fetchMetafileFromStore is the default MetafileFetcher: read
// blobs/<cid>.json from the same blob.Store the logs live in.
// Returns (nil, nil) when the metafile is missing so callers can
// distinguish "no such metafile" from "store error".
func fetchMetafileFromStore(ctx context.Context, store blob.Store, cid string) (*vod.Metafile, error) {
	key := vod.BlobsPrefix + cid + ".json"
	r, err := store.Open(ctx, key)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("open metafile %s: %w", key, err)
	}
	defer r.Close()
	body := make([]byte, r.Size())
	if _, err := r.ReadAt(body, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read metafile %s: %w", key, err)
	}
	var meta vod.Metafile
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse metafile %s: %w", key, err)
	}
	return &meta, nil
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
