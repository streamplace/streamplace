//! A small C ABI over [SlateDB](https://github.com/slatedb/slatedb), so
//! Streamplace's Go state store can keep its SQLite files in object storage.
//!
//! Go owns the semantics: which keys exist, how SQLite files are chunked, and
//! what a write means. This crate owns the async Rust around it: one
//! process-wide Tokio runtime, object store URL handling, and the mapping from
//! each Go call onto a SlateDB operation.
//!
//! The ABI is deliberately tiny and C-shaped: `i32` status (0 ok, 1 not found,
//! -1 error), out-parameters for buffers, and a single free function for every
//! allocation we hand back. Buffers are length-prefixed flat encodings rather
//! than structs with pointers, so neither side has to agree on layout or
//! ownership beyond "call `slatedb_store_free` on what you got".
//!
//! Durability: writes are acknowledged when they enter the memtable, and
//! `slatedb_store_flush` is a barrier that pushes the write-ahead log to object
//! storage and waits for it. That matches SQLite, which only assumes durability
//! at `xSync` points, and it turns a burst of page writes into one PUT instead
//! of paying a round trip per page.

use std::collections::{HashMap, HashSet};
use std::ffi::{CStr, c_char};
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::slice;
use std::sync::{Arc, LazyLock, Mutex};
use std::time::Duration;

use slatedb::config::{CloseOptions, FlushType, Settings};
use slatedb::object_store::{ObjectStore, memory::InMemory, parse_url_opts};
use slatedb::{Db, WriteBatch};
use tokio::runtime::{Builder, Runtime};

/// Status codes returned by every entry point.
pub const SLATEDB_OK: i32 = 0;
pub const SLATEDB_NOT_FOUND: i32 = 1;
pub const SLATEDB_ERROR: i32 = -1;

/// Write batch operation codes in the payload `slatedb_store_write` decodes.
const OP_PUT: u8 = 0;
const OP_DELETE: u8 = 1;

/// An open database. Opaque to Go: it only ever holds the pointer.
pub struct SlateDBStore {
    db: Db,
    /// The registry key this handle holds; released on close.
    key: String,
}

/// The runtime every SlateDB future runs on. SlateDB spawns its own background
/// tasks (compaction, flushing, manifest polling), so this must be a
/// multi-threaded runtime; the calling Go thread only drives one `block_on` at
/// a time and never occupies a worker.
static RUNTIME: LazyLock<Runtime> = LazyLock::new(|| {
    Builder::new_multi_thread()
        .enable_all()
        .thread_name("slatedb-store")
        .build()
        .expect("build tokio runtime")
});

/// SlateDB is single-writer per path: two writers racing the same database
/// corrupt it. Other processes are fenced by the manifest epoch, but a second
/// open inside *this* process is worth refusing outright, because the usual
/// cause is a mistake (a stray second connection, a test that forgot to close)
/// rather than a deliberate handover.
static OPEN_DBS: LazyLock<Mutex<HashSet<String>>> = LazyLock::new(|| Mutex::new(HashSet::new()));

fn claim(path_key: &str) -> Result<(), String> {
    let mut open = OPEN_DBS
        .lock()
        .map_err(|_| "open database registry poisoned".to_string())?;
    if open.contains(path_key) {
        return Err(format!(
            "slatedb database {path_key:?} is already open in this process"
        ));
    }
    open.insert(path_key.to_string());
    Ok(())
}

fn release(path_key: &str) {
    if let Ok(mut open) = OPEN_DBS.lock() {
        open.remove(path_key);
    }
}

/// In-memory object stores, keyed by their URL. SlateDB's in-memory store is
/// per-instance, but a caller that closes and reopens a database expects to find
/// its data again, exactly as it would for a file path. Making `memory://` URLs
/// process-wide singletons (cleared only by `slatedb_store_delete_db`) gives
/// that, and gives tests a fast stand-in for real object storage.
static MEMORY_STORES: LazyLock<Mutex<HashMap<String, Arc<dyn ObjectStore>>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));

/// Resolve an object store URL: `memory://` (shared per process) or anything
/// `object_store` understands (`s3://bucket?...`, `file:///path`, `az://...`).
///
/// Connection options travel as query parameters — `s3://bucket?endpoint=...&
/// access_key_id=...`, which is how a caller naturally writes a store URL —
/// but `object_store` wants them as an option iterator, so they are split off
/// before parsing. The URL must not carry a path: the database path is a
/// separate argument, and a prefix buried in the store URL would be silently
/// dropped.
fn resolve_object_store(store_url: &str) -> Result<Arc<dyn ObjectStore>, String> {
    let mut parsed =
        url::Url::parse(store_url).map_err(|e| format!("parse object store url: {e}"))?;

    if parsed.scheme() == "memory" {
        let mut stores = MEMORY_STORES
            .lock()
            .map_err(|_| "memory store registry poisoned".to_string())?;
        let store = stores
            .entry(store_url.to_string())
            .or_insert_with(|| Arc::new(InMemory::new()) as Arc<dyn ObjectStore>);
        return Ok(Arc::clone(store));
    }

    let mut options: Vec<(String, String)> = parsed
        .query_pairs()
        .map(|(k, v)| (k.into_owned(), v.into_owned()))
        .collect();

    // A store reached over http:// is an unencrypted one, which object_store
    // refuses unless allow_http says so. Writing the endpoint in the clear is
    // the decision, so the option follows from it; a caller who wants
    // something else can still set allow_http itself.
    if !options.iter().any(|(key, _)| key == "allow_http") {
        let http = options.iter().any(|(key, value)| {
            key == "endpoint" && value.to_ascii_lowercase().starts_with("http://")
        });
        if http {
            options.push(("allow_http".to_string(), "true".to_string()));
        }
    }

    parsed.set_query(None);

    let (store, path) = parse_url_opts(&parsed, options)
        .map_err(|e| format!("open object store {store_url:?} (options may be missing): {e}"))?;
    if !path.as_ref().is_empty() {
        return Err(format!(
            "object store url must not have a path (got {path:?}); the database path is a separate argument"
        ));
    }
    Ok(Arc::from(store))
}

// ---------------------------------------------------------------------------
// Flat buffer encodings
//
// Every buffer crossing the ABI is a self-contained byte string: no pointers,
// no struct layout, nothing to keep alive. Lengths are little-endian u32.
//
//   write batch  [u32 ops][u8 op][u32 key len][u32 value len][key][value]...
//   scan result  [u32 rows][u32 key len][key][u32 value len][value]...
// ---------------------------------------------------------------------------

struct Reader<'a> {
    buf: &'a [u8],
    pos: usize,
}

impl<'a> Reader<'a> {
    fn u8(&mut self) -> Result<u8, String> {
        let bytes = self.bytes(1)?;
        Ok(bytes[0])
    }

    fn u32(&mut self) -> Result<u32, String> {
        let bytes = self.bytes(4)?;
        Ok(u32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]))
    }

    fn bytes(&mut self, len: usize) -> Result<&'a [u8], String> {
        let end = self
            .pos
            .checked_add(len)
            .ok_or_else(|| "length overflows buffer".to_string())?;
        if end > self.buf.len() {
            return Err(format!(
                "truncated buffer: need {len} bytes at offset {}, have {}",
                self.pos,
                self.buf.len() - self.pos
            ));
        }
        let out = &self.buf[self.pos..end];
        self.pos = end;
        Ok(out)
    }

    fn done(&self) -> Result<(), String> {
        if self.pos != self.buf.len() {
            return Err(format!(
                "buffer has {} trailing bytes",
                self.buf.len() - self.pos
            ));
        }
        Ok(())
    }
}

fn put_u32(out: &mut Vec<u8>, value: u32) {
    out.extend_from_slice(&value.to_le_bytes());
}

/// Hand ownership of a byte buffer to Go. It must come back through
/// `slatedb_store_free`.
unsafe fn out_bytes(value: Vec<u8>, out: *mut *mut u8, out_len: *mut usize) {
    let len = value.len();
    let ptr = Box::into_raw(value.into_boxed_slice()) as *mut u8;
    unsafe {
        if !out.is_null() {
            *out = ptr;
        }
        if !out_len.is_null() {
            *out_len = len;
        }
    }
}

unsafe fn set_err(err: *mut *mut u8, err_len: *mut usize, msg: String) {
    unsafe { out_bytes(msg.into_bytes(), err, err_len) }
}

unsafe fn read_cstr(ptr: *const c_char, what: &str) -> Result<String, String> {
    if ptr.is_null() {
        return Err(format!("{what} is null"));
    }
    unsafe { CStr::from_ptr(ptr) }
        .to_str()
        .map(|s| s.to_string())
        .map_err(|e| format!("{what} is not valid UTF-8: {e}"))
}

/// Run a fallible body, turning a panic into an error status. A panic must never
/// unwind into Go's C frames, and a SlateDB bug deep in the stack should surface
/// as a normal database error the caller can report, not a dead process.
fn guarded<F>(err: *mut *mut u8, err_len: *mut usize, out: *mut *mut SlateDBStore, f: F) -> i32
where
    F: FnOnce() -> Result<i32, String>,
{
    let result = catch_unwind(AssertUnwindSafe(f));
    match result {
        Ok(Ok(status)) => status,
        Ok(Err(msg)) => {
            unsafe { set_err(err, err_len, msg) };
            SLATEDB_ERROR
        }
        Err(panic) => {
            let msg = match panic.downcast_ref::<&str>() {
                Some(s) => (*s).to_string(),
                None => match panic.downcast_ref::<String>() {
                    Some(s) => s.clone(),
                    None => "panic".to_string(),
                },
            };
            let msg = format!("slatedb panicked: {msg}");
            unsafe {
                // A panic may have happened anywhere in the open path, so make
                // sure the caller is left with no half-built handle.
                if !out.is_null() {
                    *out = std::ptr::null_mut();
                }
                set_err(err, err_len, msg);
            }
            SLATEDB_ERROR
        }
    }
}

/// Open (creating if needed) the database at `path` in the object store
/// described by `store_url`. `flush_interval_ms` is how long an un-synced write
/// may sit before SlateDB pushes the write-ahead log to object storage anyway;
/// zero means the SlateDB default.
///
/// # Safety
/// `store_url` and `path` are NUL-terminated UTF-8 strings; `out`, `err`, and
/// `err_len` are valid pointers (the latter two may be null to discard errors).
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_open(
    store_url: *const c_char,
    path: *const c_char,
    flush_interval_ms: u64,
    out: *mut *mut SlateDBStore,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, out, || {
        let store_url = unsafe { read_cstr(store_url, "object store url") }?;
        let path = unsafe { read_cstr(path, "database path") }?;
        if path.is_empty() {
            return Err("database path is empty".to_string());
        }
        if out.is_null() {
            return Err("out handle is null".to_string());
        }
        // The store comes first: a bad object store URL fails here, and a
        // failure must not leave the path claimed (which would refuse the
        // next open with a "already open in this process" that is not true).
        let store = resolve_object_store(&store_url)?;
        let key = format!("{store_url}\n{path}");
        claim(&key)?;

        let mut settings = Settings::default();
        if flush_interval_ms > 0 {
            settings.flush_interval = Some(Duration::from_millis(flush_interval_ms));
        }

        let db = RUNTIME.block_on(async {
            Db::builder(path.clone(), store)
                .with_settings(settings)
                .build()
                .await
        });
        let db = match db {
            Ok(db) => db,
            Err(e) => {
                release(&key);
                return Err(format!("open slatedb database {path:?}: {e}"));
            }
        };

        unsafe {
            *out = Box::into_raw(Box::new(SlateDBStore { db, key }));
        }
        Ok(SLATEDB_OK)
    })
}

/// Read one key. Values are copied into a fresh buffer, or `SLATEDB_NOT_FOUND`
/// is returned with no buffer.
///
/// # Safety
/// `handle` came from `slatedb_store_open`; `key`/`key_len` describe a byte
/// string; `out`/`out_len` are writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_get(
    handle: *const SlateDBStore,
    key: *const u8,
    key_len: usize,
    out: *mut *mut u8,
    out_len: *mut usize,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let store = unsafe { handle.as_ref() }.ok_or("null database handle")?;
        let key = unsafe { slice_from_raw(key, key_len, "key") }?;
        let value = RUNTIME
            .block_on(store.db.get(key))
            .map_err(|e| format!("get: {e}"))?;
        match value {
            Some(value) => {
                unsafe { out_bytes(value.to_vec(), out, out_len) };
                Ok(SLATEDB_OK)
            }
            None => Ok(SLATEDB_NOT_FOUND),
        }
    })
}

/// Apply a write batch (see the payload layout at the top of this file). Callers
/// that need the write in object storage before continuing must follow up with
/// `slatedb_store_flush`.
///
/// # Safety
/// As `get`, with `payload`/`payload_len` describing the encoded batch.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_write(
    handle: *const SlateDBStore,
    payload: *const u8,
    payload_len: usize,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let store = unsafe { handle.as_ref() }.ok_or("null database handle")?;
        let payload = unsafe { slice_from_raw(payload, payload_len, "write batch") }?;

        let mut reader = Reader {
            buf: payload,
            pos: 0,
        };
        let count = reader.u32()? as usize;
        let mut batch = WriteBatch::new();
        for i in 0..count {
            let op = reader.u8()?;
            let key_len = reader.u32()? as usize;
            let value_len = reader.u32()? as usize;
            let key = reader.bytes(key_len)?;
            let value = reader.bytes(value_len)?;
            if key.is_empty() {
                return Err(format!("write batch operation {i} has an empty key"));
            }
            match op {
                OP_PUT => batch.put(key, value),
                OP_DELETE => batch.delete(key),
                other => {
                    return Err(format!(
                        "write batch operation {i} has unknown code {other}"
                    ));
                }
            }
        }
        reader.done()?;

        // Awaiting `write` means the batch is admitted and visible to our own
        // reads (the memtable); durability waits for the next flush barrier.
        RUNTIME
            .block_on(store.db.write(batch))
            .map_err(|e| format!("write: {e}"))?;
        Ok(SLATEDB_OK)
    })
}

/// Scan every key with `prefix`, returning the encoded row list.
///
/// # Safety
/// As `get`, with `prefix`/`prefix_len` describing the key prefix.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_scan_prefix(
    handle: *const SlateDBStore,
    prefix: *const u8,
    prefix_len: usize,
    out: *mut *mut u8,
    out_len: *mut usize,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let store = unsafe { handle.as_ref() }.ok_or("null database handle")?;
        let prefix = unsafe { slice_from_raw(prefix, prefix_len, "prefix") }?;

        let encoded = RUNTIME.block_on(async {
            // An empty start bound: every key is >= the empty byte string, so
            // this is the whole prefix range.
            let start: &[u8] = b"";
            let mut iter = store
                .db
                .scan_prefix(prefix, start..)
                .await
                .map_err(|e| format!("scan: {e}"))?;
            let mut rows: Vec<(Vec<u8>, Vec<u8>)> = Vec::new();
            while let Some(row) = iter.next().await.map_err(|e| format!("scan: {e}"))? {
                rows.push((row.key.to_vec(), row.value.to_vec()));
            }
            let mut encoded = Vec::new();
            put_u32(&mut encoded, rows.len() as u32);
            for (key, value) in rows {
                put_u32(&mut encoded, key.len() as u32);
                encoded.extend_from_slice(&key);
                put_u32(&mut encoded, value.len() as u32);
                encoded.extend_from_slice(&value);
            }
            Ok::<Vec<u8>, String>(encoded)
        })?;

        unsafe { out_bytes(encoded, out, out_len) };
        Ok(SLATEDB_OK)
    })
}

/// Delete every key with `prefix`. Used when a state database is thrown away;
/// SlateDB has no per-key bulk delete, so this is a scan and a batch, applied in
/// chunks so a large directory does not have to fit in one memtable flush.
///
/// # Safety
/// As `scan_prefix`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_delete_prefix(
    handle: *const SlateDBStore,
    prefix: *const u8,
    prefix_len: usize,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let store = unsafe { handle.as_ref() }.ok_or("null database handle")?;
        let prefix = unsafe { slice_from_raw(prefix, prefix_len, "prefix") }?;

        RUNTIME.block_on(async {
            let start: &[u8] = b"";
            let mut iter = store
                .db
                .scan_prefix(prefix, start..)
                .await
                .map_err(|e| format!("delete scan: {e}"))?;
            loop {
                let mut batch = WriteBatch::new();
                let mut n = 0usize;
                while n < DELETE_BATCH_ROWS {
                    match iter.next().await.map_err(|e| format!("delete scan: {e}"))? {
                        Some(row) => {
                            batch.delete(row.key);
                            n += 1;
                        }
                        None => break,
                    }
                }
                if n == 0 {
                    break;
                }
                store
                    .db
                    .write(batch)
                    .await
                    .map_err(|e| format!("delete: {e}"))?;
            }
            Ok::<(), String>(())
        })?;

        // The deletion is only meaningful once it cannot be undone by a crash,
        // and the caller's next move is typically to close the database.
        RUNTIME
            .block_on(store.db.flush())
            .map_err(|e| format!("delete flush: {e}"))?;
        Ok(SLATEDB_OK)
    })
}

/// Rows per delete batch when dropping a prefix.
const DELETE_BATCH_ROWS: usize = 1024;

/// Wait until every write issued so far is in object storage. This is the
/// durability barrier behind SQLite's `xSync`, and also the ordering point that
/// makes writes to one file visible before a later write to another: the
/// write-ahead log is flushed in order, so a barrier after write A guarantees A
/// is stored before anything the caller issues next.
///
/// # Safety
/// `handle` came from `slatedb_store_open`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_flush(
    handle: *const SlateDBStore,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let store = unsafe { handle.as_ref() }.ok_or("null database handle")?;
        RUNTIME
            .block_on(store.db.flush())
            .map_err(|e| format!("flush: {e}"))?;
        Ok(SLATEDB_OK)
    })
}

/// Close the database, flushing outstanding writes first, and release the
/// single-writer claim. The handle must not be used afterwards.
///
/// # Safety
/// `handle` came from `slatedb_store_open` and is not used again.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_close(
    handle: *mut SlateDBStore,
    err: *mut *mut u8,
    err_len: *mut usize,
) -> i32 {
    guarded(err, err_len, std::ptr::null_mut(), || {
        let raw = handle;
        let store = unsafe { raw.as_ref() }.ok_or("null database handle")?;
        let result = RUNTIME.block_on(store.db.close_with_options(CloseOptions {
            flush_type: Some(FlushType::Wal),
        }));
        release(&store.key);
        unsafe { drop(Box::from_raw(raw)) };
        match result {
            Ok(()) => Ok(SLATEDB_OK),
            // SlateDB reports a second close as an error; for us closing an
            // already-closed database is exactly what was asked for.
            Err(e) if e.to_string().contains("already closed") => Ok(SLATEDB_OK),
            Err(e) => Err(format!("close: {e}")),
        }
    })
}

/// Free a buffer returned by any entry point. Safe to call with null.
///
/// # Safety
/// `ptr` and `len` must be exactly what an entry point returned.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn slatedb_store_free(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    drop(unsafe { Box::from_raw(slice::from_raw_parts_mut(ptr, len)) });
}

unsafe fn slice_from_raw<'a>(ptr: *const u8, len: usize, what: &str) -> Result<&'a [u8], String> {
    if ptr.is_null() {
        if len == 0 {
            return Ok(&[]);
        }
        return Err(format!("{what} is null"));
    }
    Ok(unsafe { slice::from_raw_parts(ptr, len) })
}
