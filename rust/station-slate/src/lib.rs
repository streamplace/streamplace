use std::io;
use std::ptr::NonNull;
use std::sync::{Arc, LazyLock};

use slatedb::admin::Admin;
use slatedb::bytes::Bytes;
use slatedb::object_store::aws::{AmazonS3Builder, S3ConditionalPut};
use slatedb::object_store::{ObjectStore, path::Path};
use slatedb::{CloseReason, Db, Error, ErrorKind, Settings, VersionedManifest};
use tokio::runtime::{Builder, Runtime};

static RUNTIME: LazyLock<io::Result<Runtime>> = LazyLock::new(|| {
    Builder::new_multi_thread()
        .worker_threads(2)
        .max_blocking_threads(4)
        .enable_all()
        .build()
});

fn runtime() -> Result<&'static Runtime, Status> {
    RUNTIME.as_ref().map_err(|_| Status::Failed)
}

#[repr(C)]
enum Status {
    Ok = 0,
    NotFound = 1,
    Fenced = 2,
    Invalid = 3,
    Failed = 4,
}

impl From<Error> for Status {
    fn from(error: Error) -> Self {
        // Object store errors may contain sensitive request details.
        match error.kind() {
            ErrorKind::Closed(CloseReason::Fenced) => Self::Fenced,
            ErrorKind::Invalid => Self::Invalid,
            _ => Self::Failed,
        }
    }
}

fn status(call: impl FnOnce() -> Result<(), Status>) -> Status {
    match call() {
        Ok(()) => Status::Ok,
        Err(error) => error,
    }
}

struct Engine(Arc<dyn ObjectStore>);

#[repr(C)]
struct Slice {
    data: *const u8,
    len: usize,
}

impl Slice {
    unsafe fn bytes(&self) -> Result<&[u8], Status> {
        if self.len == 0 {
            return Ok(&[]);
        }
        if self.data.is_null() || self.len > isize::MAX as usize {
            return Err(Status::Invalid);
        }
        // The caller guarantees readable, immutable storage for the call.
        Ok(unsafe { std::slice::from_raw_parts(self.data, self.len) })
    }

    unsafe fn text(&self) -> Result<&str, Status> {
        std::str::from_utf8(unsafe { self.bytes()? }).map_err(|_| Status::Invalid)
    }
}

#[repr(C)]
struct Value {
    data: Slice,
    owner: *mut Bytes,
}

fn checked<T>(ptr: *mut T) -> Result<NonNull<T>, Status> {
    NonNull::new(ptr)
        .filter(|ptr| ptr.is_aligned())
        .ok_or(Status::Invalid)
}

// The caller guarantees a live Rust handle and excludes concurrent free.
unsafe fn borrow<T>(ptr: &*const T) -> Result<&T, Status> {
    Ok(unsafe { checked(ptr.cast_mut())?.as_ref() })
}

// All entry points require the ownership and memory contract in station_slate.h.
#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_engine_new(
    endpoint: Slice,
    bucket: Slice,
    access_key: Slice,
    secret_key: Slice,
    region: Slice,
    out: *mut *mut Engine,
) -> Status {
    status(|| {
        let out = checked(out)?;
        let endpoint = unsafe { endpoint.text()? };
        let bucket = unsafe { bucket.text()? };
        let access_key = unsafe { access_key.text()? };
        let secret_key = unsafe { secret_key.text()? };
        let region = unsafe { region.text()? };
        if [endpoint, bucket, access_key, secret_key, region].contains(&"") {
            return Err(Status::Invalid);
        }
        let _runtime = runtime()?.enter();
        let store = AmazonS3Builder::new()
            .with_endpoint(endpoint)
            .with_bucket_name(bucket)
            .with_access_key_id(access_key)
            .with_secret_access_key(secret_key)
            .with_region(region)
            .with_allow_http(endpoint.starts_with("http://"))
            .with_conditional_put(S3ConditionalPut::ETagMatch)
            .build()
            .map_err(|_| Status::Invalid)?;
        let engine = Box::new(Engine(Arc::new(store)));
        unsafe { out.as_ptr().write(Box::into_raw(engine)) };
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_epoch(
    engine: *const Engine,
    path: Slice,
    out: *mut u64,
) -> Status {
    status(|| {
        let out = checked(out)?;
        let engine = unsafe { borrow(&engine)? };
        let path = Path::parse(unsafe { path.text()? }).map_err(|_| Status::Invalid)?;
        let admin = Admin::builder(path, Arc::clone(&engine.0)).build();
        let manifest = runtime()?.block_on(async {
            tokio::time::timeout(std::time::Duration::from_secs(30), admin.read_manifest(None))
                .await
                .map_err(|_| Status::Failed)?
                .map_err(Status::from)
        })?;
        let epoch = manifest.as_ref().map_or(0, VersionedManifest::writer_epoch);
        unsafe { out.as_ptr().write(epoch) };
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_open(
    engine: *const Engine,
    path: Slice,
    expected_epoch: u64,
    out: *mut *mut Db,
) -> Status {
    status(|| {
        let out = checked(out)?;
        let engine = unsafe { borrow(&engine)? };
        let path = Path::parse(unsafe { path.text()? }).map_err(|_| Status::Invalid)?;
        // object_store already retries HTTP failures. Unbounded *additional*
        // retries in the engine would keep a dead S3 request holding the Go
        // slot mutex forever, preventing orderly shutdown and later recovery.
        let settings = Settings {
            object_store_max_retries: Some(0),
            ..Settings::default()
        };
        let db = runtime()?.block_on(async {
            Db::builder(path, Arc::clone(&engine.0))
                .with_settings(settings)
                .with_expected_epoch(expected_epoch)
                .build()
                .await
        })?;
        unsafe { out.as_ptr().write(Box::into_raw(Box::new(db))) };
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_get(db: *const Db, key: Slice, out: *mut Value) -> Status {
    status(|| {
        let out = checked(out)?;
        let db = unsafe { borrow(&db)? };
        let key = unsafe { key.bytes()? };
        let bytes = runtime()?.block_on(db.get(key))?.ok_or(Status::NotFound)?;
        let data = Slice {
            data: bytes.as_ptr(),
            len: bytes.len(),
        };
        let owner = Box::into_raw(Box::new(bytes));
        unsafe { out.as_ptr().write(Value { data, owner }) };
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_put(db: *const Db, key: Slice, value: Slice) -> Status {
    status(|| {
        let db = unsafe { borrow(&db)? };
        let key = unsafe { key.bytes()? };
        let value = unsafe { value.bytes()? };
        runtime()?.block_on(async { db.put(key, value).await?.await_durable().await })?;
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_close(db: *mut Db) -> Status {
    status(|| {
        let db = checked(db)?;
        let db = unsafe { Box::from_raw(db.as_ptr()) };
        runtime()?.block_on(db.close())?;
        Ok(())
    })
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_engine_free(engine: *mut Engine) {
    if let Ok(engine) = checked(engine) {
        drop(unsafe { Box::from_raw(engine.as_ptr()) });
    }
}

#[unsafe(no_mangle)]
unsafe extern "C" fn station_slate_value_free(owner: *mut Bytes) {
    if let Ok(owner) = checked(owner) {
        drop(unsafe { Box::from_raw(owner.as_ptr()) });
    }
}
