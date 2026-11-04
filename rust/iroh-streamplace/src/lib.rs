uniffi::setup_scaffolding!();

pub mod node_addr;
pub mod public_key;

use std::sync::{LazyLock, Once};

mod socket;
pub use socket::*;

/// Lazily initialized Tokio runtime for use in uniffi methods that need a runtime.
static RUNTIME: LazyLock<tokio::runtime::Runtime> =
    LazyLock::new(|| tokio::runtime::Runtime::new().unwrap());

/// Ensure logging is only initialized once
static LOGGING_INIT: Once = Once::new();

/// Initialize logging with the default subscriber that respects RUST_LOG environment variable.
/// This function is safe to call multiple times - it will only initialize logging once.
#[uniffi::export]
pub fn init_logging() {
    LOGGING_INIT.call_once(|| {
        tracing_subscriber::fmt()
            .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
            .init();
    });
}

/// Initialize logging with a custom log level.
/// This function is safe to call multiple times - it will only initialize logging once.
///
/// # Arguments
/// * `level` - Log level as a string (e.g., "trace", "debug", "info", "warn", "error")
#[uniffi::export]
pub fn init_logging_with_level(level: String) {
    LOGGING_INIT.call_once(|| {
        let filter = tracing_subscriber::EnvFilter::try_from_default_env()
            .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new(level));

        tracing_subscriber::fmt().with_env_filter(filter).init();
    });
}
