pub mod api;
pub mod config;
pub mod db;
pub mod ingest;
pub mod metrics;
pub mod privacy;
pub mod query;

pub use config::Config;

pub mod proto {
    tonic::include_proto!("analytics");
}
