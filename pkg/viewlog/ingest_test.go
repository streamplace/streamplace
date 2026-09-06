package viewlog

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/cdn"
)

// fakeSource is an in-memory cdn.LogSource.
type fakeSource struct {
	parts    []cdn.Part
	requests map[string][]cdn.Request
	readErr  map[string]error
	reads    int
}

func (f *fakeSource) ListParts(ctx context.Context, since time.Time) ([]cdn.Part, error) {
	var out []cdn.Part
	for _, p := range f.parts {
		if !p.Day.Before(since.Truncate(24 * time.Hour)) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeSource) ReadPart(ctx context.Context, p cdn.Part, emit func(cdn.Request) error) error {
	f.reads++
	if err := f.readErr[p.ID]; err != nil {
		return err
	}
	for _, r := range f.requests[p.ID] {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

// memCursor is an in-memory IngestCursor with the same claim semantics
// as the statedb implementation.
type memCursor struct {
	mu        sync.Mutex
	claimed   map[string]time.Time
	completed map[string]int
}

func newMemCursor() *memCursor {
	return &memCursor{claimed: map[string]time.Time{}, completed: map[string]int{}}
}

func (c *memCursor) ClaimPart(ctx context.Context, source, id string, stale time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := source + "/" + id
	if _, done := c.completed[k]; done {
		return false, nil
	}
	if at, ok := c.claimed[k]; ok && time.Since(at) < stale {
		return false, nil
	}
	c.claimed[k] = time.Now()
	return true, nil
}

func (c *memCursor) CompletePart(ctx context.Context, source, id string, events int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completed[source+"/"+id] = events
	return nil
}

func (c *memCursor) ReleasePart(ctx context.Context, source, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.claimed, source+"/"+id)
	return nil
}

const blobURLBase = "https://cdn.example.com/blobs/"

func segReq(ts time.Time, status int, bytes int64, cid, sid string) cdn.Request {
	u := blobURLBase + cid + ".mp4?token=t&did=did%3Aplc%3Aalice&sid=" + sid + "&expires=1"
	return cdn.Request{Time: ts, Status: status, BytesSent: bytes, RemoteIP: "203.0.113.9", URL: u}
}

func TestSegmentEventFromRequest(t *testing.T) {
	salts := NewSaltManager(newMemSaltStorage())
	ts := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)

	ev, ok := segmentEventFromRequest(segReq(ts, 206, 1234, "bafyblob", "tid1"), salts)
	require.True(t, ok)
	require.Equal(t, Event{
		Ts: ts, Type: EventTypeSegmentRequest, SID: "tid1", IPHash: ev.IPHash,
		CID: "bafyblob", OwnerDID: "did:plc:alice", BytesSent: 1234,
	}, ev)
	require.NotEmpty(t, ev.IPHash)

	// Path-only URLs (some providers log no host) work too.
	ev, ok = segmentEventFromRequest(cdn.Request{Time: ts, Status: 200, BytesSent: 1, URL: "/blobs/bafyx.mp4?sid=tid2"}, salts)
	require.True(t, ok)
	require.Equal(t, "bafyx", ev.CID)

	// Rejected: expired-token 403s, non-blob paths, sid-less fetches.
	_, ok = segmentEventFromRequest(segReq(ts, 403, 0, "bafyblob", "tid1"), salts)
	require.False(t, ok)
	_, ok = segmentEventFromRequest(cdn.Request{Time: ts, Status: 200, URL: "https://cdn.example.com/other/bafyblob.mp4?sid=x"}, salts)
	require.False(t, ok)
	_, ok = segmentEventFromRequest(cdn.Request{Time: ts, Status: 200, URL: "https://cdn.example.com/blobs/bafyblob.mp4?did=d"}, salts)
	require.False(t, ok)
	_, ok = segmentEventFromRequest(cdn.Request{Time: ts, Status: 200, URL: "https://cdn.example.com/blobs/?sid=x"}, salts)
	require.False(t, ok)
}

func TestRunIngestBucketsAndReaggregates(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	day := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	t0 := day.Add(10*time.Hour + 2*time.Minute)

	src := &fakeSource{
		parts: []cdn.Part{
			{ID: "pullzone-logs/pz/2026/09/05_w1-0-aaaa.gzip", Day: day},
			{ID: "pullzone-logs/pz/2026/09/05_w2-0-bbbb.gzip", Day: day},
			{ID: "pullzone-logs/pz/2026/08/01_w1-0-old.gzip", Day: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)},
		},
		requests: map[string][]cdn.Request{
			"pullzone-logs/pz/2026/09/05_w1-0-aaaa.gzip": {
				segReq(t0, 206, 100, "bafyblob", "tid1"),
				segReq(t0.Add(time.Minute), 206, 100, "bafyblob", "tid1"),
				segReq(t0.Add(4*time.Minute), 206, 100, "bafyblob", "tid1"), // next 5m window
				segReq(t0, 403, 0, "bafyblob", "tid9"),                      // skipped
			},
			"pullzone-logs/pz/2026/09/05_w2-0-bbbb.gzip": {
				segReq(t0.Add(time.Second), 200, 50, "bafyother", "tid2"),
			},
		},
	}
	cursor := newMemCursor()
	type reagg struct {
		part       string
		start, end time.Time
	}
	var reaggs []reagg
	in := IngestInput{
		Store: store, Source: src, SourceName: "bunny", Salts: NewSaltManager(newMemSaltStorage()),
		Cursor: cursor, Window: 5 * time.Minute, Lookback: 48 * time.Hour,
		Reaggregate: func(ctx context.Context, part string, start, end time.Time) error {
			reaggs = append(reaggs, reagg{part, start, end})
			return nil
		},
		Now: func() time.Time { return now },
	}
	res, err := RunIngest(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 2, res.PartsListed, "the August part is outside the 48h lookback")
	require.Equal(t, 2, res.PartsIngested)
	require.Equal(t, 4, res.Events)
	require.Equal(t, 1, res.Skipped)
	require.Equal(t, 3, res.Windows, "part 1 spans two windows, part 2 one")

	// Output files: one per (part, window), named by window start so
	// the aggregator's filename-timestamp selection finds them.
	keys, err := store.List(context.Background(), viewLogsPrefix)
	require.NoError(t, err)
	sort.Strings(keys)
	w0 := t0.Truncate(5 * time.Minute)
	w1 := w0.Add(5 * time.Minute)
	require.Equal(t, []string{
		"view-logs/cdn-bunny/pullzone-logs_pz_2026_09_05_w1-0-aaaa.gzip/" + w0.Format(keyTimeFormat) + ".jsonl.gz",
		"view-logs/cdn-bunny/pullzone-logs_pz_2026_09_05_w1-0-aaaa.gzip/" + w1.Format(keyTimeFormat) + ".jsonl.gz",
		"view-logs/cdn-bunny/pullzone-logs_pz_2026_09_05_w2-0-bbbb.gzip/" + w0.Format(keyTimeFormat) + ".jsonl.gz",
	}, keys)
	for _, k := range keys {
		ts, ok := parseViewLogKeyTime(k)
		require.True(t, ok, k)
		require.False(t, ts.Before(w0))
	}

	// Each touched window is handed back for re-aggregation, with the
	// part in the request so the task key is unique per part.
	require.Equal(t, []reagg{
		{"pullzone-logs/pz/2026/09/05_w1-0-aaaa.gzip", w0, w1},
		{"pullzone-logs/pz/2026/09/05_w1-0-aaaa.gzip", w1, w1.Add(5 * time.Minute)},
		{"pullzone-logs/pz/2026/09/05_w2-0-bbbb.gzip", w0, w1},
	}, reaggs)

	// The events round-trip through the aggregator: tid1 fetched three
	// segments across two windows, tid2 one.
	var n int
	for _, k := range keys {
		require.NoError(t, readJSONLGz(context.Background(), store, k, func(ev *Event) {
			n++
			require.Equal(t, EventTypeSegmentRequest, ev.Type)
			require.NotZero(t, ev.BytesSent)
			require.Zero(t, ev.RangeEnd)
		}))
	}
	require.Equal(t, 4, n)

	// A second run finds nothing new: every part is complete.
	res, err = RunIngest(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 0, res.PartsIngested)
	require.Equal(t, 2, src.reads, "completed parts are not re-read")
}

func TestRunIngestFailedPartIsReleasedAndRetried(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	day := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	src := &fakeSource{
		parts:   []cdn.Part{{ID: "p/bad.gzip", Day: day}, {ID: "p/good.gzip", Day: day}},
		readErr: map[string]error{"p/bad.gzip": errors.New("boom")},
		requests: map[string][]cdn.Request{
			"p/good.gzip": {segReq(day.Add(time.Hour), 200, 10, "bafyblob", "tid1")},
			"p/bad.gzip":  {segReq(day.Add(time.Hour), 200, 10, "bafyblob", "tid1")},
		},
	}
	cursor := newMemCursor()
	in := IngestInput{
		Store: store, Source: src, SourceName: "bunny", Salts: NewSaltManager(newMemSaltStorage()),
		Cursor: cursor, Window: 5 * time.Minute,
		Now: func() time.Time { return day.Add(26 * time.Hour) },
	}
	res, err := RunIngest(context.Background(), in)
	require.Error(t, err, "the bad part's error surfaces after every part is attempted")
	require.Equal(t, 1, res.PartsIngested, "the good part still landed")

	keys, err := store.List(context.Background(), viewLogsPrefix)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Contains(t, keys[0], "p_good.gzip")

	// The failure released the claim; once the source recovers, the
	// next run ingests it.
	delete(src.readErr, "p/bad.gzip")
	res, err = RunIngest(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 1, res.PartsIngested)
	keys, err = store.List(context.Background(), viewLogsPrefix)
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestIngestTaskKeys(t *testing.T) {
	start := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	require.NotEqual(t, AggregateTaskKey(start, end), ReaggregateTaskKey(start, end, "p/a.gzip"))
	require.NotEqual(t, ReaggregateTaskKey(start, end, "p/a.gzip"), ReaggregateTaskKey(start, end, "p/b.gzip"))
	require.Equal(t, "cdn-log-ingest::2026-09-05T10:00:00Z", IngestTaskKey(start))
}
