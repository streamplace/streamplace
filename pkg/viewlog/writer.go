package viewlog

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/log"
)

// Writer buffers Event values gzipped in memory and periodically
// uploads the buffer as a single object to its backing blob.Store.
// Each upload's key is the writer's keyPrefix + the buffer's opened-at
// timestamp + ".jsonl.gz", so files for a window are easy to list +
// time-order at aggregation time.
//
// Log is fast (one mutex, one JSON encode into a gzip writer); the
// upload happens off the request goroutine. A Run loop drives time-
// based flushes; size-based flushes are nudged via flushReq.
type Writer struct {
	store      blob.Store
	keyPrefix  string
	flushAfter time.Duration
	maxBytes   int
	salts      *SaltManager
	now        func() time.Time

	mu             sync.Mutex
	buf            *bytes.Buffer
	gz             *gzip.Writer
	openedAt       time.Time
	eventCount     int
	unflushedBytes int // pre-compression; gzip's internal buffer hides post-compression size until Close

	flushReq chan struct{}
	closeCh  chan struct{}
	doneCh   chan struct{}
}

// Config bundles writer settings. NodeDID identifies the writer in the
// key layout: `view-logs/<NodeDID>/<window>.jsonl.gz`. A CDN ETL would
// use the same shape with a CDN-side identifier.
type Config struct {
	Store      blob.Store
	NodeDID    string
	FlushAfter time.Duration
	MaxBytes   int
	Salts      *SaltManager
	// Now is an optional clock override for tests. Defaults to time.Now.
	Now func() time.Time
}

func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("viewlog: Store is required")
	}
	if cfg.NodeDID == "" {
		return nil, fmt.Errorf("viewlog: NodeDID is required")
	}
	if cfg.FlushAfter <= 0 {
		cfg.FlushAfter = 5 * time.Minute
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 10 * 1024 * 1024
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	w := &Writer{
		store:      cfg.Store,
		keyPrefix:  fmt.Sprintf("%s%s/", viewLogsPrefix, cfg.NodeDID),
		flushAfter: cfg.FlushAfter,
		maxBytes:   cfg.MaxBytes,
		salts:      cfg.Salts,
		now:        cfg.Now,
		flushReq:   make(chan struct{}, 1),
		closeCh:    make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	w.reset()
	return w, nil
}

// Salts returns the writer's SaltManager. Handlers use it to hash IPs
// before logging.
func (w *Writer) Salts() *SaltManager { return w.salts }

// reset opens a fresh in-memory gzip buffer. Called from the writer
// loop while the mutex is held.
func (w *Writer) reset() {
	w.buf = new(bytes.Buffer)
	w.gz = gzip.NewWriter(w.buf)
	w.openedAt = w.now()
	w.eventCount = 0
	w.unflushedBytes = 0
}

// Log encodes ev into the current buffer and nudges a flush if the
// uncompressed byte count crossed maxBytes. We track pre-compression
// bytes because gzip's internal buffer makes w.buf.Len() useless as a
// size signal between flushes. Errors here are logged and swallowed
// — a playback handler can't usefully recover from a view-log write
// failure, and propagating would change the playback contract.
func (w *Writer) Log(ctx context.Context, ev Event) {
	line, err := json.Marshal(&ev)
	if err != nil {
		log.Error(ctx, "viewlog: encode event", "error", err, "type", ev.Type)
		return
	}
	line = append(line, '\n')

	w.mu.Lock()
	_, werr := w.gz.Write(line)
	if werr == nil {
		w.unflushedBytes += len(line)
		w.eventCount++
	}
	overSize := w.unflushedBytes >= w.maxBytes
	w.mu.Unlock()
	if werr != nil {
		log.Error(ctx, "viewlog: gzip write", "error", werr, "type", ev.Type)
		return
	}
	if overSize {
		select {
		case w.flushReq <- struct{}{}:
		default:
			// A flush is already queued; nothing to do.
		}
	}
}

// Run blocks until ctx is cancelled or Close is called, periodically
// (every flushAfter / 2) checking for an open buffer with events and
// flushing it. Size-based flushes arrive on flushReq from Log.
func (w *Writer) Run(ctx context.Context) {
	defer close(w.doneCh)
	tick := time.NewTicker(w.flushAfter)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			w.flushNow(context.Background(), "context-done")
			return
		case <-w.closeCh:
			w.flushNow(context.Background(), "close")
			return
		case <-tick.C:
			w.flushNow(ctx, "tick")
		case <-w.flushReq:
			w.flushNow(ctx, "size")
		}
	}
}

// Close signals Run to flush and exit, then waits for it.
func (w *Writer) Close() error {
	select {
	case <-w.closeCh:
		// Already closed.
	default:
		close(w.closeCh)
	}
	<-w.doneCh
	return nil
}

// flushNow uploads whatever's currently buffered as a single
// .jsonl.gz blob; a no-op if the buffer is empty. Reason is recorded
// on the log line for ops visibility (size vs tick vs close).
func (w *Writer) flushNow(ctx context.Context, reason string) {
	w.mu.Lock()
	if w.eventCount == 0 {
		w.mu.Unlock()
		return
	}
	oldBuf := w.buf
	oldGz := w.gz
	oldOpenedAt := w.openedAt
	oldCount := w.eventCount
	w.reset()
	w.mu.Unlock()

	if err := oldGz.Close(); err != nil {
		log.Error(ctx, "viewlog: gzip close", "error", err)
		return
	}
	key := w.keyPrefix + oldOpenedAt.UTC().Format(keyTimeFormat) + ".jsonl.gz"
	writer, err := w.store.NewWriter(ctx, key, "application/gzip")
	if err != nil {
		log.Error(ctx, "viewlog: open store writer", "error", err, "key", key)
		return
	}
	if _, err := writer.Write(oldBuf.Bytes()); err != nil {
		log.Error(ctx, "viewlog: write blob", "error", err, "key", key)
		_ = writer.Close()
		return
	}
	if err := writer.Complete(); err != nil {
		log.Error(ctx, "viewlog: complete blob", "error", err, "key", key)
		return
	}
	log.Debug(ctx, "viewlog flushed",
		"key", key,
		"events", oldCount,
		"bytes", oldBuf.Len(),
		"reason", reason,
	)
}
