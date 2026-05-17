// Package blob defines a content-agnostic blob storage interface and
// supplies file-system and S3 implementations.
//
// The interface is deliberately small — Open / NewWriter / Move /
// Delete — so it can serve as a building block for higher-level
// stores: an S3 archive backed by a local disk cache backed by an
// in-memory cache, for instance. Implementations all assume random
// access on reads (every Reader is an io.ReaderAt with a known Size)
// because the upstream consumers (gstreamer demuxers, range requests
// from playback clients) need it.
//
// Keys are forward-slash-separated strings interpreted relative to the
// store's root (a directory for FileStore, a bucket for S3Store).
// Implementations must accept arbitrary nested paths; the FileStore
// auto-creates parent directories at write time.
package blob

import (
	"context"
	"errors"
	"io"
)

// Store is a content-agnostic random-access blob storage backend.
//
// Implementations are safe for concurrent use from multiple goroutines.
// Reads via Open are completely independent of each other and of any
// in-flight writes to the same key. Behavior when a write to a key
// races a delete or a move of the same key is implementation-defined
// — callers should serialize those operations externally.
type Store interface {
	// URL returns a human-readable URL for the given key, useful for
	// logging and error messages. Format is implementation-specific
	// (e.g. "file:///abs/path", "s3://bucket/key").
	URL(key string) string

	// Open returns a Reader for the blob at key. The supplied context
	// scopes any underlying network requests (S3) and is checked when
	// closing the reader (the reader holds it). The returned Reader's
	// Size method returns the byte length of the blob.
	Open(ctx context.Context, key string) (Reader, error)

	// NewWriter starts a streaming write to key. Bytes written go to
	// implementation-private staging until Complete is called; on
	// Complete the blob becomes visible at key atomically. Abort
	// (or Close without Complete) discards the staged bytes.
	//
	// contentType is advisory and may be ignored by implementations
	// that don't track it (FileStore).
	NewWriter(ctx context.Context, key, contentType string) (Writer, error)

	// Move relocates the blob from srcKey to dstKey atomically (where
	// the underlying storage permits — POSIX rename on FileStore,
	// CopyObject+DeleteObject on S3Store). If dstKey already exists,
	// it is overwritten. Returns nil if srcKey does not exist after a
	// successful Move (idempotency for retried renames).
	Move(ctx context.Context, srcKey, dstKey string) error

	// Delete removes the blob at key. Returns nil if the blob does
	// not exist (the desired post-condition is the same either way).
	Delete(ctx context.Context, key string) error

	// ParseLocation translates a backend-specific URL or path (as
	// stored in legacy fields like the upload row's Location column)
	// into a Store-relative key. Returns ok=false if the location
	// doesn't belong to this Store (wrong scheme, wrong bucket, path
	// outside the configured root).
	ParseLocation(location string) (key string, ok bool)

	// List enumerates every key whose forward-slash-relative-to-root
	// path starts with prefix. The prefix is treated as a literal
	// string match against keys (matches the S3 ListObjectsV2 prefix
	// semantics, not a directory boundary), so "foo/bar" matches
	// "foo/bar.json" and "foo/bar/baz". An empty prefix lists every
	// key. The internal staging area (if any) is excluded. Order is
	// implementation-defined; callers should sort if they need it.
	List(ctx context.Context, prefix string) ([]string, error)
}

// Reader is the random-access read half of a blob. Implementations
// must support concurrent ReadAt calls (file ReaderAt is safe; the
// S3-backed implementation serializes internally).
type Reader interface {
	io.ReaderAt
	io.Closer
	// Size returns the byte length of the blob, known at Open time.
	Size() int64
}

// Writer is the streaming-write half of a blob. Writes go to staging
// until Complete (atomic publish to the configured key) or Abort
// (discard). Close runs Abort if Complete hasn't been called, so a
// `defer w.Close()` after NewWriter is the recommended error-path
// guard.
type Writer interface {
	io.Writer
	// Complete publishes the staged bytes to the configured key. After
	// a successful Complete the blob exists at key; the writer cannot
	// be reused.
	Complete() error
	// Abort discards staged bytes. Safe to call multiple times and
	// safe to call after Complete (in which case it's a no-op).
	Abort() error
	// Close calls Abort if Complete has not been called.
	io.Closer
}

// ErrNotFound is returned by Open when the requested key doesn't
// exist. Implementations should wrap their backend's not-found error
// (os.ErrNotExist, s3 NoSuchKey) so callers can test with errors.Is.
var ErrNotFound = errors.New("blob: not found")
