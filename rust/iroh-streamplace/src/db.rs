use std::{
    fmt::{self, Debug},
    ops::Bound,
    pin::Pin,
    sync::Arc,
    time::Duration,
};

use bytes::Bytes;
use n0_future::{Stream, StreamExt};
use ref_cast::RefCast;
use snafu::Snafu;
use tokio::sync::Mutex;

use crate::public_key::PublicKey;

// the files here are just copied from iroh-smol-kv-uniffi/src/code
mod kv {
    mod time_bound;
    pub use time_bound::TimeBound;
    mod subscribe_mode;
    pub use subscribe_mode::SubscribeMode;
}
pub use kv::{SubscribeMode, TimeBound};
use util::format_bytes;

/// Error creating a new database node.
#[derive(Debug, Snafu, uniffi::Error)]
#[snafu(module)]
pub enum CreateError {
    /// The provided private key is invalid (not 32 bytes).
    PrivateKey { size: u64 },
    /// The provided gossip topic is invalid (not 32 bytes).
    Topic { size: u64 },
    /// Failed to bind the iroh endpoint.
    Bind { message: String },
    /// Failed to subscribe to the gossip topic.
    Subscribe { message: String },
}

/// Error joining peers.
#[derive(Debug, Snafu, uniffi::Error)]
#[snafu(module)]
pub enum JoinPeersError {
    /// Failed to parse a provided iroh node ticket.
    Ticket { message: String },
    /// Error during the join peers operation.
    Irpc { message: String },
}

/// Error joining peers.
#[derive(Debug, Snafu, uniffi::Error)]
#[snafu(module)]
pub enum ParseError {
    /// Failed to parse a provided iroh node ticket.
    Ticket { message: String },
}

/// Error putting a value into the database.
#[derive(Debug, Snafu, uniffi::Error)]
#[snafu(module)]
pub enum PutError {
    /// Error during the put operation.
    Irpc { message: String },
}

/// Configuration for an iroh-streamplace node.
#[derive(uniffi::Record, Clone)]
pub struct Config {
    /// An Ed25519 secret key as a 32 byte array.
    pub key: Vec<u8>,
    /// The gossip topic to use. Must be 32 bytes.
    ///
    /// You can use e.g. a BLAKE3 hash of a topic string here. This can be used
    /// as a cheap way to have a shared secret - nodes that do not know the topic
    /// cannot connect to the swarm.
    pub topic: Vec<u8>,
    /// Maximum duration to wait for sending a stream piece to a peer.
    pub max_send_duration: Duration,
    /// Disable using relays, for tests.
    pub disable_relay: bool,
}

#[derive(uniffi::Enum, Debug, Clone)]
enum StreamFilter {
    All,
    Global,
    Stream(Vec<u8>),
}

/// A filter for subscriptions and iteration.
#[derive(uniffi::Object, Debug, Clone)]
pub struct Filter {
    scope: Option<Vec<Arc<PublicKey>>>,
    stream: StreamFilter,
    min_time: TimeBound,
    max_time: TimeBound,
}

#[uniffi::export]
impl Filter {
    /// Creates a new filter that matches everything.
    #[uniffi::constructor]
    pub fn new() -> Arc<Self> {
        Arc::new(Self {
            scope: None,
            stream: StreamFilter::All,
            min_time: TimeBound::Unbounded,
            max_time: TimeBound::Unbounded,
        })
    }

    /// Restrict to the global namespace, no per stream data.
    pub fn global(mut self: Arc<Self>) -> Arc<Self> {
        let this = Arc::make_mut(&mut self);
        this.stream = StreamFilter::Global;
        self
    }

    /// Restrict to one specific stream, no global data.
    pub fn stream(mut self: Arc<Self>, stream: Vec<u8>) -> Arc<Self> {
        let this = Arc::make_mut(&mut self);
        this.stream = StreamFilter::Stream(stream);
        self
    }

    /// Restrict to a set of scopes.
    pub fn scopes(mut self: Arc<Self>, scopes: Vec<Arc<PublicKey>>) -> Arc<Self> {
        let this = Arc::make_mut(&mut self);
        this.scope = Some(scopes);
        self
    }

    /// Restrict to a single scope.
    pub fn scope(self: Arc<Self>, scope: Arc<PublicKey>) -> Arc<Self> {
        self.scopes(vec![scope])
    }

    /// Restrict to a time range.
    pub fn timestamps(mut self: Arc<Self>, min: TimeBound, max: TimeBound) -> Arc<Self> {
        let this = Arc::make_mut(&mut self);
        this.min_time = min;
        this.max_time = max;
        self
    }

    /// Restrict to a time range given in nanoseconds since unix epoch.
    pub fn time_range(self: Arc<Self>, min: u64, max: u64) -> Arc<Self> {
        self.timestamps(TimeBound::Included(min), TimeBound::Excluded(max))
    }

    /// Restrict to a time range starting at min, unbounded at the top.
    pub fn time_from(self: Arc<Self>, min: u64) -> Arc<Self> {
        self.timestamps(TimeBound::Included(min), TimeBound::Unbounded)
    }
}

impl From<Filter> for iroh_smol_kv::Filter {
    fn from(value: Filter) -> Self {
        let mut filter = iroh_smol_kv::Filter::ALL;
        match value.stream {
            // everything
            StreamFilter::All => {}
            // everything starting with 'g', for the global namespace
            StreamFilter::Global => {
                filter = filter.key_prefix(b"g".as_ref());
            }
            // a specific stream, everything starting with 's' + escaped stream name
            StreamFilter::Stream(t) => {
                let prefix = util::encode_stream_and_key(Some(&t), &[]);
                filter = filter.key_prefix(prefix);
            }
        };
        filter = filter.timestamps_nanos((
            Bound::<u64>::from(value.min_time),
            Bound::<u64>::from(value.max_time),
        ));
        if let Some(scopes) = value.scope {
            let keys = scopes.iter().map(|k| iroh::PublicKey::from(k.as_ref()));
            filter = filter.scopes(keys);
        }
        filter
    }
}

/// Error getting the next item from a subscription.
#[derive(uniffi::Enum, Snafu, Debug)]
#[snafu(module)]
pub enum SubscribeNextError {
    /// Error during the subscribe next operation.
    Irpc { message: String },
}

/// Error getting the next item from a subscription.
#[derive(uniffi::Enum, Snafu, Debug)]
#[snafu(module)]
pub enum WriteError {
    /// The provided private key is invalid (not 32 bytes).
    PrivateKeySize { size: u64 },
}

/// Error shutting down the database.
///
/// This can occur if the db is already shut down or if there is an internal error.
#[derive(uniffi::Enum, Snafu, Debug)]
#[snafu(module)]
pub enum ShutdownError {
    /// Error during the shutdown operation.
    Irpc { message: String },
}

/// An entry returned from the database.
#[derive(uniffi::Record, Debug, PartialEq, Eq)]
pub struct Entry {
    scope: Arc<PublicKey>,
    stream: Option<Vec<u8>>,
    key: Vec<u8>,
    value: Vec<u8>,
    timestamp: u64,
}

/// An item returned from a subscription.
#[derive(uniffi::Enum, Debug)]
pub enum SubscribeItem {
    Entry {
        scope: Arc<PublicKey>,
        stream: Option<Vec<u8>>,
        key: Vec<u8>,
        value: Vec<u8>,
        timestamp: u64,
    },
    CurrentDone,
    Expired {
        scope: Arc<PublicKey>,
        stream: Option<Vec<u8>>,
        key: Vec<u8>,
        timestamp: u64,
    },
    Other,
}

fn fmt_stream(stream: &Option<Vec<u8>>) -> String {
    match stream {
        None => "<nil>".to_string(),
        Some(s) => format_bytes(s),
    }
}

#[uniffi::export]
pub fn subscribe_item_debug(item: &SubscribeItem) -> String {
    match item {
        SubscribeItem::Entry {
            scope,
            stream,
            key,
            value,
            timestamp,
        } => format!(
            "Entry {{ scope: {}, stream: {}, key: {}, value: {}, timestamp: {} }}",
            scope.fmt_short(),
            fmt_stream(stream),
            format_bytes(key),
            format_bytes(value),
            timestamp
        ),
        SubscribeItem::CurrentDone => "CurrentDone".to_string(),
        SubscribeItem::Expired {
            scope,
            stream,
            key,
            timestamp,
        } => format!(
            "Expired {{ scope: {}, stream: {}, key: {}, timestamp: {} }}",
            scope.fmt_short(),
            fmt_stream(stream),
            format_bytes(key),
            timestamp
        ),
        SubscribeItem::Other => "Other".to_string(),
    }
}

impl From<iroh_smol_kv::SubscribeItem> for SubscribeItem {
    fn from(item: iroh_smol_kv::SubscribeItem) -> Self {
        match &item {
            iroh_smol_kv::SubscribeItem::Entry((scope, key, value)) => {
                let Some((stream, key)) = util::decode_stream_and_key(key) else {
                    return Self::Other;
                };
                Self::Entry {
                    scope: Arc::new((*scope).into()),
                    stream,
                    key,
                    value: value.value.to_vec(),
                    timestamp: value.timestamp,
                }
            }
            iroh_smol_kv::SubscribeItem::CurrentDone => Self::CurrentDone,
            iroh_smol_kv::SubscribeItem::Expired((scope, topic, timestamp)) => {
                let (stream, key) = util::decode_stream_and_key(topic).unwrap();
                Self::Expired {
                    scope: Arc::new((*scope).into()),
                    stream,
                    key,
                    timestamp: *timestamp,
                }
            }
        }
    }
}

/// A response to a subscribe request.
///
/// This can be used as a stream of [`SubscribeItem`]s.
#[derive(uniffi::Object)]
#[uniffi::export(Debug)]
#[allow(clippy::type_complexity)]
pub struct SubscribeResponse {
    inner: Mutex<
        Pin<
            Box<
                dyn Stream<Item = Result<iroh_smol_kv::SubscribeItem, irpc::Error>>
                    + Send
                    + Sync
                    + 'static,
            >,
        >,
    >,
}

impl fmt::Debug for SubscribeResponse {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("SubscribeResponse").finish_non_exhaustive()
    }
}

#[uniffi::export]
impl SubscribeResponse {
    pub async fn next_raw(&self) -> Result<Option<SubscribeItem>, SubscribeNextError> {
        let mut this = self.inner.lock().await;
        match this.as_mut().next().await {
            None => Ok(None),
            Some(Ok(item)) => Ok(Some(item.into())),
            Some(Err(e)) => Err(SubscribeNextError::Irpc {
                message: e.to_string(),
            }),
        }
    }
}

/// Options for subscribing.
///
/// `filter` specifies what to subscribe to.
/// `mode` specifies whether to get current items, new items, or both.
#[derive(uniffi::Record)]
pub struct SubscribeOpts {
    pub filter: Arc<Filter>,
    pub mode: SubscribeMode,
}

impl From<SubscribeOpts> for iroh_smol_kv::Subscribe {
    fn from(opts: SubscribeOpts) -> Self {
        iroh_smol_kv::Subscribe {
            filter: opts.filter.as_ref().clone().into(),
            mode: opts.mode.into(),
        }
    }
}

/// A write scope that can be used to put values into the database.
///
/// The default write scope is available from the [`Node::node_scope`] method.
#[derive(Clone, Debug, RefCast, uniffi::Object)]
#[repr(transparent)]
pub struct WriteScope(iroh_smol_kv::WriteScope);

#[uniffi::export]
impl WriteScope {
    pub async fn put(
        &self,
        stream: Option<Vec<u8>>,
        key: Vec<u8>,
        value: Vec<u8>,
    ) -> Result<(), PutError> {
        self.put_impl(stream, key, value.into())
            .await
            .map_err(|e| PutError::Irpc {
                message: e.to_string(),
            })
    }
}

impl WriteScope {
    pub fn new(inner: iroh_smol_kv::WriteScope) -> Self {
        Self(inner)
    }

    /// Put a value into the database, optionally in a specific stream.
    pub async fn put_impl(
        &self,
        stream: Option<impl AsRef<[u8]>>,
        key: impl AsRef<[u8]>,
        value: Bytes,
    ) -> Result<(), irpc::Error> {
        let key = key.as_ref();
        let stream = stream.as_ref().map(|s| s.as_ref());
        let encoded = util::encode_stream_and_key(stream, key);
        self.0.put(encoded, value).await?;
        Ok(())
    }
}

/// Iroh-streamplace specific metadata database.
#[derive(Debug, Clone, RefCast, uniffi::Object)]
#[repr(transparent)]
pub struct Db(iroh_smol_kv::Client);

impl Db {
    pub fn new(inner: iroh_smol_kv::Client) -> Self {
        Self(inner)
    }

    pub fn inner(&self) -> &iroh_smol_kv::Client {
        &self.0
    }
}

#[uniffi::export]
impl Db {
    pub fn write(&self, secret: Vec<u8>) -> Result<Arc<WriteScope>, WriteError> {
        let secret = iroh::SecretKey::from_bytes(&secret.try_into().map_err(|e: Vec<u8>| {
            WriteError::PrivateKeySize {
                size: e.len() as u64,
            }
        })?);
        let write = self.0.write(secret);
        Ok(Arc::new(WriteScope::new(write)))
    }

    pub async fn iter_with_opts(
        &self,
        filter: Arc<Filter>,
    ) -> Result<Vec<Entry>, SubscribeNextError> {
        let sub = self.subscribe_with_opts(SubscribeOpts {
            filter,
            mode: SubscribeMode::Current,
        });
        let mut items = Vec::new();
        while let Some(item) = sub.next_raw().await? {
            match item {
                SubscribeItem::Entry {
                    scope,
                    stream,
                    key,
                    value,
                    timestamp,
                } => {
                    items.push(Entry {
                        scope,
                        stream,
                        key,
                        value,
                        timestamp,
                    });
                }
                _ => unreachable!("we used SubscribeMode::Current, so we should only get entries"),
            }
        }
        Ok(items)
    }

    pub fn subscribe(&self, filter: Arc<Filter>) -> Arc<SubscribeResponse> {
        self.subscribe_with_opts(SubscribeOpts {
            filter,
            mode: SubscribeMode::Both,
        })
    }

    /// Subscribe with options.
    pub fn subscribe_with_opts(&self, opts: SubscribeOpts) -> Arc<SubscribeResponse> {
        Arc::new(SubscribeResponse {
            inner: Mutex::new(Box::pin(
                self.0.subscribe_with_opts(opts.into()).stream_raw(),
            )),
        })
    }

    /// Shutdown the database client and all subscriptions.
    pub async fn shutdown(&self) -> Result<(), ShutdownError> {
        Ok(self.0.shutdown().await.map_err(|e| ShutdownError::Irpc {
            message: e.to_string(),
        })?)
    }
}

mod util {

    pub fn encode_stream_and_key(stream: Option<&[u8]>, key: &[u8]) -> Vec<u8> {
        let mut result = Vec::new();
        if let Some(s) = stream {
            result.push(b's');
            postcard::to_io(s, &mut result).unwrap();
        } else {
            result.push(b'g');
        }
        result.extend(key);
        result
    }

    pub fn decode_stream_and_key(encoded: &[u8]) -> Option<(Option<Vec<u8>>, Vec<u8>)> {
        match encoded.split_first() {
            Some((b's', mut rest)) => {
                let (stream, rest) = postcard::take_from_bytes(&mut rest).ok()?;
                Some((Some(stream), rest.to_vec()))
            }
            Some((b'g', rest)) => Some((None, rest.to_vec())),
            _ => None,
        }
    }

    pub fn format_bytes(bytes: &[u8]) -> String {
        if bytes.is_empty() {
            return "\"\"".to_string();
        }
        let Ok(s) = std::str::from_utf8(bytes) else {
            return hex::encode(bytes);
        };
        if s.chars()
            .any(|c| c.is_control() && c != '\n' && c != '\t' && c != '\r')
        {
            return hex::encode(bytes);
        }
        format!("\"{}\"", escape_string(s))
    }

    pub fn escape_string(s: &str) -> String {
        s.chars()
            .map(|c| match c {
                '"' => "\\\"".to_string(),
                '\\' => "\\\\".to_string(),
                '\n' => "\\n".to_string(),
                '\t' => "\\t".to_string(),
                '\r' => "\\r".to_string(),
                c => c.to_string(),
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::util;

    #[test]
    fn escape_unescape() {
        let cases: Vec<(Option<&[u8]>, &[u8])> = vec![
            (None, b"key1"),
            (Some(b""), b""),
            (Some(b""), b"a"),
            (Some(b"a"), b""),
        ];
        for (stream, key) in cases {
            let encoded = util::encode_stream_and_key(stream, key);
            let (decoded_stream, decoded_key) = util::decode_stream_and_key(&encoded).unwrap();
            assert_eq!(decoded_stream.as_deref(), stream);
            assert_eq!(decoded_key.as_slice(), key);
        }
    }
}
