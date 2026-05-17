package viewlog

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/blob"
)

// readAllJSONL gunzips and JSON-decodes every line in the given file.
func readAllJSONL(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gz.Close()
	var out []Event
	sc := bufio.NewScanner(gz)
	for sc.Scan() {
		var ev Event
		require.NoError(t, json.Unmarshal(sc.Bytes(), &ev))
		out = append(out, ev)
	}
	require.NoError(t, sc.Err())
	return out
}

// listViewLogKeys walks the file store under the writer's prefix and
// returns the relative keys, sorted by filename (which is RFC3339-style
// so lex order == time order).
func listViewLogKeys(t *testing.T, root, nodeDID string) []string {
	t.Helper()
	dir := filepath.Join(root, "view-logs", nodeDID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out
}

func TestWriterFlushOnClose(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	w, err := NewWriter(Config{
		Store:      store,
		NodeDID:    "did:web:test.example",
		FlushAfter: 1 * time.Hour, // won't fire during the test
		Salts:      NewSaltManager(newMemSaltStorage()),
		Now:        func() time.Time { return now },
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	w.Log(ctx, Event{Ts: now, Type: EventTypeManifestRequest, VideoURI: "at://x/place.stream.video/1", SID: "abc"})
	w.Log(ctx, Event{Ts: now.Add(time.Second), Type: EventTypeSegmentRequest, CID: "bafyfoo", SID: "abc"})

	require.NoError(t, w.Close())

	keys := listViewLogKeys(t, root, "did:web:test.example")
	require.Len(t, keys, 1, "exactly one flush on Close")
	got := readAllJSONL(t, keys[0])
	require.Len(t, got, 2)
	require.Equal(t, EventTypeManifestRequest, got[0].Type)
	require.Equal(t, "at://x/place.stream.video/1", got[0].VideoURI)
	require.Equal(t, EventTypeSegmentRequest, got[1].Type)
	require.Equal(t, "bafyfoo", got[1].CID)
}

func TestWriterFlushOnSize(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)

	// Tight size cap so every event nudges flushReq. We assert that
	// every event lands SOMEWHERE on disk (no data loss); how many
	// mid-stream flushes the Run loop actually performed depends on
	// the Go scheduler, so the only guarantee is one final flush at
	// Close.
	w, err := NewWriter(Config{
		Store:      store,
		NodeDID:    "did:web:test.example",
		FlushAfter: 1 * time.Hour,
		MaxBytes:   64,
		Salts:      NewSaltManager(newMemSaltStorage()),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	for i := 0; i < 40; i++ {
		w.Log(ctx, Event{
			Ts:       time.Now().UTC(),
			Type:     EventTypeSegmentRequest,
			CID:      "bafy_segment_with_a_reasonably_long_cid_string_so_we_overflow",
			SID:      "3jw5xxxxxxxxx",
			IPHash:   "deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef",
			OwnerDID: "did:plc:abcdefghijklmnop",
		})
	}

	require.NoError(t, w.Close())
	keys := listViewLogKeys(t, root, "did:web:test.example")
	require.GreaterOrEqual(t, len(keys), 1, "Close must flush whatever's buffered")

	var total int
	for _, k := range keys {
		total += len(readAllJSONL(t, k))
	}
	require.Equal(t, 40, total, "every event lands in some flush — no data loss")
}

func TestWriterNoOpWhenEmpty(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)

	w, err := NewWriter(Config{
		Store:      store,
		NodeDID:    "did:web:test.example",
		FlushAfter: 1 * time.Hour,
		Salts:      NewSaltManager(newMemSaltStorage()),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	require.NoError(t, w.Close())
	keys := listViewLogKeys(t, root, "did:web:test.example")
	require.Empty(t, keys, "no events ⇒ no files")
}

func TestWriterConcurrentLogs(t *testing.T) {
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)

	w, err := NewWriter(Config{
		Store:      store,
		NodeDID:    "did:web:test.example",
		FlushAfter: 1 * time.Hour,
		Salts:      NewSaltManager(newMemSaltStorage()),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	const goroutines = 8
	const perGoroutine = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				w.Log(ctx, Event{
					Ts:   time.Now().UTC(),
					Type: EventTypeManifestRequest,
					SID:  "sid",
				})
			}
		}()
	}
	wg.Wait()

	require.NoError(t, w.Close())
	keys := listViewLogKeys(t, root, "did:web:test.example")
	var total int
	for _, k := range keys {
		total += len(readAllJSONL(t, k))
	}
	require.Equal(t, goroutines*perGoroutine, total)
}
