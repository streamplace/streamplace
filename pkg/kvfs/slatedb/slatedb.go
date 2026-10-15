// Package slatedb is a kvfs.Store backed by [SlateDB], whose write-ahead log
// and tables live in object storage instead of on a local disk. A streamplace
// state database opened through it is the same SQLite schema (see pkg/statedb)
// with its files chunked into a key/value database in a bucket.
//
// The database is named by a URL of the form
//
//	slatedb+<object store>://<bucket or name>/<path>[?options]
//
// where the object store half is anything SlateDB's object_store understands —
// `slatedb+s3://bucket/state?endpoint=http://localhost:9000&access_key_id=...`
// for an S3-compatible service, `slatedb+memory://name/state` for an in-memory
// store that lasts as long as the process (tests). The path after the bucket is
// the database's prefix within the store, so several databases can share one
// bucket.
//
// [SlateDB]: https://github.com/slatedb/slatedb
package slatedb

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust/slatedb-store/include
#include <stdlib.h>
#include "slatedb_store.h"
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unsafe"

	"stream.place/streamplace/pkg/kvfs"
	// The C ABI's link flags live in the streamplacedeps pkg-config file,
	// like every other Rust library this repo links.
	_ "stream.place/streamplace/pkg/streamplacedeps"
)

// Operation codes in the write batches handed to the C ABI; they must match
// OP_PUT and OP_DELETE in rust/slatedb-store/src/lib.rs.
const (
	opPut    = 0
	opDelete = 1
)

// status codes from slatedb_store.h.
const (
	statusOK       = 0
	statusNotFound = 1
)

// Options configures an open database.
type Options struct {
	// FlushInterval bounds how long writes may sit in the write-ahead log
	// before SlateDB pushes it to object storage anyway. Zero uses SlateDB's
	// default. Callers that need a write durable before a specific moment
	// should ask for it with Flush rather than tuning this.
	FlushInterval time.Duration
}

// Store is a kvfs.Store over one SlateDB database. It is safe for concurrent
// use: every call blocks the calling goroutine on the shared Rust runtime and
// owns the buffers it passes.
type Store struct {
	handle *C.SlateDBStore

	// name is the URL the store was opened from, for error messages.
	name string
}

var _ kvfs.Store = (*Store)(nil)

// Open opens (creating if it does not exist) the database named by rawURL.
func Open(rawURL string, opts Options) (*Store, error) {
	storeURL, path, err := parseURL(rawURL)
	if err != nil {
		return nil, err
	}
	cStoreURL, err := cString(storeURL)
	if err != nil {
		return nil, fmt.Errorf("slatedb: opening %s: object store url: %w", rawURL, err)
	}
	defer C.free(unsafe.Pointer(cStoreURL))
	cPath, err := cString(path)
	if err != nil {
		return nil, fmt.Errorf("slatedb: opening %s: database path: %w", rawURL, err)
	}
	defer C.free(unsafe.Pointer(cPath))

	var (
		handle *C.SlateDBStore
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_open(cStoreURL, cPath, C.uint64_t(opts.FlushInterval.Milliseconds()), &handle, &errPtr, &errLen)
	if status != statusOK {
		return nil, fmt.Errorf("slatedb: opening %s: %w", rawURL, cerror(errPtr, errLen))
	}
	return &Store{handle: handle, name: rawURL}, nil
}

// Get returns the value stored under key, or false if the key is not there.
func (s *Store) Get(key []byte) ([]byte, bool, error) {
	var (
		out    *C.uint8_t
		outLen C.size_t
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_get(s.handle, bytePtr(key), C.size_t(len(key)), &out, &outLen, &errPtr, &errLen)
	switch status {
	case statusOK:
		value := C.GoBytes(unsafe.Pointer(out), C.int(outLen))
		C.slatedb_store_free(out, outLen)
		return value, true, nil
	case statusNotFound:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("slatedb: %s: reading key %q: %w", s.name, key, cerror(errPtr, errLen))
	}
}

// Write applies mutations in one batch: a value of nil deletes the key, which
// is how kvfs drops chunks. The write is visible to reads immediately and
// durable once Flush returns.
func (s *Store) Write(mutations []kvfs.Mutation) error {
	if len(mutations) == 0 {
		return nil
	}
	payload := encodeBatch(mutations)
	var (
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_write(s.handle, bytePtr(payload), C.size_t(len(payload)), &errPtr, &errLen)
	if status != statusOK {
		return fmt.Errorf("slatedb: %s: writing %d mutations: %w", s.name, len(mutations), cerror(errPtr, errLen))
	}
	return nil
}

// Scan returns every key with prefix, in key order.
func (s *Store) Scan(prefix []byte) ([]kvfs.Row, error) {
	var (
		out    *C.uint8_t
		outLen C.size_t
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_scan_prefix(s.handle, bytePtr(prefix), C.size_t(len(prefix)), &out, &outLen, &errPtr, &errLen)
	if status != statusOK {
		return nil, fmt.Errorf("slatedb: %s: scanning prefix %q: %w", s.name, prefix, cerror(errPtr, errLen))
	}
	payload := C.GoBytes(unsafe.Pointer(out), C.int(outLen))
	C.slatedb_store_free(out, outLen)
	return decodeRows(payload)
}

// DeletePrefix removes every key with prefix.
func (s *Store) DeletePrefix(prefix []byte) error {
	var (
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_delete_prefix(s.handle, bytePtr(prefix), C.size_t(len(prefix)), &errPtr, &errLen)
	if status != statusOK {
		return fmt.Errorf("slatedb: %s: deleting prefix %q: %w", s.name, prefix, cerror(errPtr, errLen))
	}
	return nil
}

// Flush waits until every write issued so far is in object storage.
func (s *Store) Flush() error {
	var (
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_flush(s.handle, &errPtr, &errLen)
	if status != statusOK {
		return fmt.Errorf("slatedb: %s: flushing: %w", s.name, cerror(errPtr, errLen))
	}
	return nil
}

// Close flushes outstanding writes, closes the database, and releases the
// single-writer claim this process holds on it. The store must not be used
// afterwards.
func (s *Store) Close() error {
	if s.handle == nil {
		return nil
	}
	var (
		errPtr *C.uint8_t
		errLen C.size_t
	)
	status := C.slatedb_store_close(s.handle, &errPtr, &errLen)
	s.handle = nil
	if status != statusOK {
		return fmt.Errorf("slatedb: %s: closing: %w", s.name, cerror(errPtr, errLen))
	}
	return nil
}

// parseURL splits a slatedb+ URL into the object store URL SlateDB's
// object_store should open and the database's path within it.
func parseURL(rawURL string) (storeURL, path string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("slatedb: parsing %q: %w", rawURL, err)
	}
	scheme, ok := strings.CutPrefix(u.Scheme, "slatedb+")
	if !ok || scheme == "" {
		return "", "", fmt.Errorf("slatedb: url %q must start with slatedb+ and an object store, like slatedb+s3://bucket/state", rawURL)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("slatedb: url %q names no bucket: the part after the scheme is the object store's bucket or name", rawURL)
	}
	path = strings.Trim(u.Path, "/")
	if path == "" {
		return "", "", fmt.Errorf("slatedb: url %q names no database: the path after the bucket is the database's prefix within it", rawURL)
	}
	// The options travel with the bucket; the path is SlateDB's business.
	store := url.URL{Scheme: scheme, Host: u.Host, RawQuery: u.RawQuery}
	return store.String(), path, nil
}

// encodeBatch writes the flat write-batch encoding the C ABI decodes:
// [u32 ops][u8 op][u32 key len][u32 value len][key][value]...
func encodeBatch(mutations []kvfs.Mutation) []byte {
	size := 4
	for _, m := range mutations {
		size += 1 + 4 + 4 + len(m.Key) + len(m.Value)
	}
	out := make([]byte, 4, size)
	binary.LittleEndian.PutUint32(out, uint32(len(mutations)))
	for _, m := range mutations {
		op := byte(opPut)
		if m.Value == nil {
			op = opDelete
		}
		out = append(out, op)
		out = binary.LittleEndian.AppendUint32(out, uint32(len(m.Key)))
		out = binary.LittleEndian.AppendUint32(out, uint32(len(m.Value)))
		out = append(out, m.Key...)
		out = append(out, m.Value...)
	}
	return out
}

// decodeRows reads the flat scan encoding:
// [u32 rows][u32 key len][key][u32 value len][value]...
func decodeRows(payload []byte) ([]kvfs.Row, error) {
	read := func(what string, n int, pos *int) ([]byte, error) {
		end := *pos + n
		if n < 0 || end > len(payload) {
			return nil, fmt.Errorf("slatedb: scanned rows are malformed: %s needs %d bytes at offset %d of %d", what, n, *pos, len(payload))
		}
		out := payload[*pos:end]
		*pos = end
		return out, nil
	}
	if len(payload) < 4 {
		return nil, fmt.Errorf("slatedb: scanned rows are malformed: %d bytes, want at least 4", len(payload))
	}
	pos := 0
	rows, err := read("row count", 4, &pos)
	if err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint32(rows))
	out := make([]kvfs.Row, 0, count)
	for i := range count {
		keyLen, err := read(fmt.Sprintf("length of key %d", i), 4, &pos)
		if err != nil {
			return nil, err
		}
		key, err := read(fmt.Sprintf("key %d", i), int(binary.LittleEndian.Uint32(keyLen)), &pos)
		if err != nil {
			return nil, err
		}
		valueLen, err := read(fmt.Sprintf("length of value %d", i), 4, &pos)
		if err != nil {
			return nil, err
		}
		value, err := read(fmt.Sprintf("value %d", i), int(binary.LittleEndian.Uint32(valueLen)), &pos)
		if err != nil {
			return nil, err
		}
		out = append(out, kvfs.Row{Key: key, Value: value})
	}
	if pos != len(payload) {
		return nil, fmt.Errorf("slatedb: scanned rows are malformed: %d trailing bytes", len(payload)-pos)
	}
	return out, nil
}

// cerror takes ownership of an error buffer the C ABI handed back and turns it
// into a Go error. Every failing call sets one; a missing one means the C layer
// failed without saying why, which is worth reporting as such.
func cerror(ptr *C.uint8_t, length C.size_t) error {
	if ptr == nil {
		return fmt.Errorf("the store reported no error message")
	}
	defer C.slatedb_store_free(ptr, length)
	return fmt.Errorf("%s", C.GoStringN((*C.char)(unsafe.Pointer(ptr)), C.int(length)))
}

// cString copies a Go string into C memory, which the caller must free,
// rejecting strings with embedded NULs.
func cString(s string) (*C.char, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return nil, fmt.Errorf("%q contains a NUL byte", s)
	}
	return C.CString(s), nil
}

// bytePtr passes a Go byte slice to C for the duration of one call. C must not
// keep the pointer, which the ABI's synchronous, copy-on-return calls honor.
func bytePtr(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(b)))
}
