// Package kvfs is a SQLite VFS that keeps a database's files in a key/value
// store instead of on a filesystem, so a SQLite database can live in object
// storage alongside the media Streamplace already keeps there.
//
// One store holds one database, and inside it each SQLite file (the database,
// its write-ahead log, a rollback journal) is a directory of keys:
//
//	len/<file>             the file's logical length, 8 bytes big endian
//	chunk/<file>/<8 hex>   one chunk of file data, always ChunkSize bytes
//
// Chunks are stored whole, so a page-sized write replaces exactly one value.
// The length lives in its own key so an empty file is distinguishable from a
// missing one: SQLite asks that question to decide whether a journal needs
// rolling back.
//
// Writes are collected per file and handed to the store at xSync, which is the
// only point SQLite requires them to be durable. That turns a transaction's
// dozens of page writes into one batch, and the store's flush barrier into the
// fsync SQLite thinks it is getting. The barrier also orders files against each
// other: a write issued after a barrier lands after it, which is what makes a
// committed journal durable before the pages it protects.
//
// The VFS has no shared-memory methods, so SQLite is put into exclusive locking
// mode (DSN) to use the write-ahead log out of heap memory, as it does for any
// VFS without xShmMap.
package kvfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/psanford/sqlite3vfs"
	"stream.place/streamplace/pkg/log"
)

// Store is the key/value backend that file data lives in. pkg/kvfs/slatedb
// implements it against SlateDB; tests use an in-memory fake.
type Store interface {
	// Get returns the value stored for key, or ok false when it is absent.
	Get(key []byte) (value []byte, ok bool, err error)

	// Write applies mutations in order, as one atomic batch. A mutation with
	// a nil Value deletes its key.
	Write(mutations []Mutation) error

	// Scan returns every row whose key begins with prefix, ordered by key.
	Scan(prefix []byte) ([]Row, error)

	// DeletePrefix removes every key that begins with prefix.
	DeletePrefix(prefix []byte) error

	// Flush returns once every write issued so far is durable in the backing
	// store, not merely visible to this process.
	Flush() error
}

// Mutation is one key/value change in a Store batch.
type Mutation struct {
	Key []byte
	// Value replaces the key's contents; nil deletes the key.
	Value []byte
}

// Row is one key/value pair returned by a Store scan.
type Row struct {
	Key   []byte
	Value []byte
}

const (
	// DefaultChunkSize is the unit file data is stored in. It matches SQLite's
	// default page size, so a page rewrite touches exactly one stored value.
	DefaultChunkSize = 4096

	// DefaultCacheBytes is how much chunk data a VFS keeps in memory. SQLite
	// asks for the pages it just wrote back out again (journal checks, index
	// lookups), and those are the chunks this holds.
	DefaultCacheBytes = 8 << 20
)

// Options tune a VFS. The zero value is the defaults.
type Options struct {
	// ChunkSize overrides DefaultChunkSize.
	ChunkSize int
	// CacheBytes overrides DefaultCacheBytes.
	CacheBytes int
}

// vfsSerial names registered VFSes uniquely: sqlite3vfs registers one VFS per
// name for the life of the process, and a process may hold several stores
// (one per state database, plus tests).
var vfsSerial atomic.Uint64

// VFS serves SQLite files out of one Store.
type VFS struct {
	name      string
	store     Store
	chunkSize int64
	cache     *cache

	// mu covers everything below it and every file operation. SQLite calls
	// this VFS from one connection at a time (mattn's driver holds a lock
	// around the connection, and a state database is capped at one
	// connection), so a single mutex costs nothing and leaves no room for
	// lock-order mistakes between the cache and its callers.
	mu    sync.Mutex
	files map[string][]*file
}

// New registers a VFS serving store and returns it. The VFS name is generated,
// so several stores can be live at once; DSN builds the SQLite data source
// name that opens a file through it.
func New(store Store, opts Options) (*VFS, error) {
	if store == nil {
		return nil, fmt.Errorf("kvfs: no store")
	}
	chunkSize := int64(opts.ChunkSize)
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}
	cacheBytes := opts.CacheBytes
	if cacheBytes == 0 {
		cacheBytes = DefaultCacheBytes
	}
	v := &VFS{
		name:      fmt.Sprintf("kvfs-%d", vfsSerial.Add(1)),
		store:     store,
		chunkSize: chunkSize,
		files:     map[string][]*file{},
	}
	v.cache = newCache(cacheBytes, chunkSize)
	if err := sqlite3vfs.RegisterVFS(v.name, v); err != nil {
		return nil, fmt.Errorf("kvfs: registering vfs: %w", err)
	}
	return v, nil
}

// Close releases the store's resources if it holds any. A SlateDB store holds
// a claim as the database's single writer, and a process that wants to reopen
// the same database (a test, or a restart in one process) has to give that
// claim back. Files still open through the VFS become unusable; close the
// database first.
func (v *VFS) Close() error {
	if closer, ok := v.store.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Name is the SQLite VFS name this store is registered under.
func (v *VFS) Name() string { return v.name }

// DSN is the data source name that opens name through this VFS. Exclusive
// locking is what makes the write-ahead log possible without shared memory, so
// it is set here rather than left to whoever opens the database; synchronous
// FULL is what makes SQLite ask for a sync at every commit, which is when the
// store's flush barrier runs.
func (v *VFS) DSN(name string) string {
	return fmt.Sprintf("file:%s?vfs=%s&_locking_mode=exclusive&_synchronous=full", name, v.name)
}

// Open implements sqlite3vfs.VFS. An empty name is SQLite's request for a
// temporary file, which is served from memory: temp data is scratch by
// definition and has no business in object storage.
func (v *VFS) Open(name string, flags sqlite3vfs.OpenFlag) (sqlite3vfs.File, sqlite3vfs.OpenFlag, error) {
	if name == "" {
		return newMemFile(), flags, nil
	}
	if err := checkName(name); err != nil {
		return nil, 0, sqlite3vfs.CantOpenError
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	f, err := v.openLocked(name, flags)
	if err != nil {
		return nil, 0, err
	}
	v.files[name] = append(v.files[name], f)
	return f, flags, nil
}

func (v *VFS) openLocked(name string, flags sqlite3vfs.OpenFlag) (*file, error) {
	var (
		size   int64
		exists bool
	)
	value, ok, err := v.store.Get(lengthKey(name))
	if err != nil {
		return nil, v.ioError("reading length of "+name, err)
	}
	if ok {
		if len(value) != 8 {
			return nil, v.ioError("length of "+name, fmt.Errorf("malformed length value (%d bytes)", len(value)))
		}
		size = int64(binary.BigEndian.Uint64(value))
		exists = true
	}
	if !exists {
		if flags&sqlite3vfs.OpenCreate == 0 {
			// Opening a file that is not there, without asking to create it.
			return nil, sqlite3vfs.CantOpenError
		}
		size = 0
	} else if flags&sqlite3vfs.OpenExclusive != 0 {
		// SQLITE_OPEN_EXCLUSIVE asks for a file that is not there yet: what
		// it is meant to create is already in the store, so this open fails
		// rather than handing SQLite a file it did not expect to find.
		return nil, sqlite3vfs.CantOpenError
	}
	f := &file{
		vfs:       v,
		name:      name,
		size:      size,
		readOnly:  flags&sqlite3vfs.OpenReadOnly != 0,
		allocated: chunksFor(size, v.chunkSize),
		dirty:     map[uint32]bool{},
	}
	if !exists {
		// Record the empty file right away: a file SQLite creates exists from
		// this point on, and it asks again (xAccess) before trusting that.
		if err := f.write([]Mutation{{Key: lengthKey(name), Value: encodeLength(0)}}); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// Delete implements sqlite3vfs.VFS: SQLite removing a file, most often the
// write-ahead log or a journal it no longer needs.
func (v *VFS) Delete(name string, dirSync bool) error {
	if name == "" {
		return nil
	}
	if err := checkName(name); err != nil {
		return sqlite3vfs.IOError
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, ok, err := v.store.Get(lengthKey(name)); err != nil {
		return v.ioError("looking up "+name, err)
	} else if !ok {
		return sqlite3vfs.NotFoundError
	}
	if err := v.store.DeletePrefix(chunkPrefix(name)); err != nil {
		return v.ioError("deleting chunks of "+name, err)
	}
	if err := v.store.Write([]Mutation{{Key: lengthKey(name)}}); err != nil {
		return v.ioError("deleting "+name, err)
	}
	v.cache.forget(name)
	return nil
}

// Access implements sqlite3vfs.VFS.
func (v *VFS) Access(name string, flag sqlite3vfs.AccessFlag) (bool, error) {
	if name == "" {
		return false, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	_, ok, err := v.store.Get(lengthKey(name))
	if err != nil {
		return false, v.ioError("looking up "+name, err)
	}
	if flag == sqlite3vfs.AccessExists {
		return ok, nil
	}
	// Any store this VFS can write is readable and writable.
	return true, nil
}

// FullPathname implements sqlite3vfs.VFS. Names are already canonical: the
// store is the whole filesystem.
func (v *VFS) FullPathname(name string) string {
	if err := checkName(name); err != nil {
		return ""
	}
	return name
}

// removeFileLocked drops a file from the open registry; the VFS mutex is held.
func (v *VFS) removeFileLocked(f *file) {
	open := v.files[f.name]
	for i, other := range open {
		if other == f {
			v.files[f.name] = append(open[:i], open[i+1:]...)
			break
		}
	}
	if len(v.files[f.name]) == 0 {
		delete(v.files, f.name)
	}
}

// lockHeld reports whether some other open file on the same name holds a lock
// that conflicts with want. SQLite is the only writer of a store's files, so
// this only ever arbitrates between files open in this process.
func (v *VFS) lockHeld(f *file, want sqlite3vfs.LockType) bool {
	for _, other := range v.files[f.name] {
		if other == f {
			continue
		}
		switch want {
		case sqlite3vfs.LockNone:
			return false
		case sqlite3vfs.LockShared:
			// Shared is compatible with shared, and nothing stronger.
			if other.lock > sqlite3vfs.LockShared {
				return true
			}
		default:
			if other.lock >= sqlite3vfs.LockShared {
				return true
			}
		}
	}
	return false
}

// reservedElsewhere answers SQLite's question about whether a reserved or
// stronger lock is held by someone else, which is how a reader decides a write
// is in progress.
func (v *VFS) reservedElsewhere(f *file) bool {
	for _, other := range v.files[f.name] {
		if other == f {
			continue
		}
		if other.lock >= sqlite3vfs.LockReserved {
			return true
		}
	}
	return false
}

// ioError wraps a store failure in SQLite's IO error and adds the context the
// sentinel alone would drop: SQLite only reports "disk I/O error" to the app,
// so the real cause has to be said out loud here.
func (v *VFS) ioError(what string, err error) error {
	log.Warn(context.Background(), "kvfs: store failure", "what", what, "error", err)
	return sqlite3vfs.IOError
}

func lengthKey(name string) []byte { return []byte("len/" + name) }

func chunkPrefix(name string) []byte { return []byte("chunk/" + name + "/") }

func chunkKey(name string, index uint32) []byte {
	return []byte("chunk/" + name + "/" + fmt.Sprintf("%08x", index))
}

func encodeLength(size int64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(size))
	return buf[:]
}

// chunksFor is how many chunks a file of this many bytes occupies. A
// zero-length file occupies none: its length key is what says it exists.
func chunksFor(size, chunkSize int64) uint32 {
	if size <= 0 {
		return 0
	}
	return uint32((size + chunkSize - 1) / chunkSize)
}

// checkName rejects names that could escape the store's key space. SQLite hands
// back what was put in the DSN plus its own suffixes ("-wal", "-journal"), so
// anything odd is a bug worth refusing rather than storing.
func checkName(name string) error {
	if name == "" || strings.ContainsAny(name, "\x00/") || name == "." || name == ".." {
		return fmt.Errorf("kvfs: bad file name %q", name)
	}
	return nil
}

// secretQueryParams are the query parameters a database URL can carry
// credentials in. An object store URL is written with its keys in the query
// string (see pkg/kvfs/slatedb for the slatedb+ form), and a URL like that does
// not stay in one place: it is parsed into the store's configuration and then
// quoted back by the client when something fails, so it has to be redacted
// before it reaches a log line or an error, not after.
var secretQueryParams = []string{"secret_access_key", "session_token", "token", "password"}

// RedactURLSecrets returns a database URL with the values of its credential
// query parameters replaced: the only form of it that may be kept for an error
// message or a log line. A URL that will not parse is replaced outright, since
// there is no way to tell what it was carrying.
func RedactURLSecrets(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unparseable database URL"
	}
	// The query is taken apart by hand rather than through url.Values: a
	// single bad escape anywhere in it makes the whole map come back short,
	// and the parameter that falls out of it must not be the one with the
	// credentials in it.
	fields := strings.Split(u.RawQuery, "&")
	redacted := false
	for i, field := range fields {
		name, _, ok := strings.Cut(field, "=")
		if !ok || !isSecretParam(name) {
			continue
		}
		fields[i] = name + "=redacted"
		redacted = true
	}
	if !redacted {
		return rawURL
	}
	u.RawQuery = strings.Join(fields, "&")
	return u.String()
}

// URLSecrets returns the credentials a database URL carries in its query
// string, for taking them back out of text that came from somewhere else: the
// object store's client is free to quote the URL it was given, and those errors
// reach logs. A value is returned both as it was written and decoded, when that
// is different: a quoted URL may have been through either form.
func URLSecrets(rawURL string) []string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	var secrets []string
	for _, field := range strings.Split(u.RawQuery, "&") {
		name, value, ok := strings.Cut(field, "=")
		if !ok || !isSecretParam(name) {
			continue
		}
		secrets = append(secrets, value)
		if decoded, err := url.QueryUnescape(value); err == nil && decoded != value {
			secrets = append(secrets, decoded)
		}
	}
	return secrets
}

// isSecretParam reports whether a query parameter name carries a credential.
// The name is also compared decoded, which is how the object store's own URL
// parser reads it.
func isSecretParam(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return slices.Contains(secretQueryParams, name)
}
