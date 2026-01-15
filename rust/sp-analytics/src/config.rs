use figment::{
    Figment,
    providers::{Env, Format, Toml},
};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub server: ServerConfig,
    pub clickhouse: ClickHouseConfig,
    pub wal: WalConfig,
    pub ingestion: IngestionConfig,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ServerConfig {
    #[serde(default = "default_grpc_port")]
    pub grpc_port: u16,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ClickHouseConfig {
    #[serde(default = "default_clickhouse_url")]
    pub url: String,
    #[serde(default = "default_database")]
    pub database: String,
    #[serde(default)]
    pub username: String,
    #[serde(default)]
    pub password: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct WalConfig {
    #[serde(default = "default_wal_path")]
    pub path: String,
    #[serde(default = "default_wal_enabled")]
    pub enabled: bool,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct IngestionConfig {
    #[serde(default = "default_batch_size")]
    pub batch_size: usize,
    #[serde(default = "default_flush_interval_ms")]
    pub flush_interval_ms: u64,
    #[serde(default = "default_max_retry_attempts")]
    pub max_retry_attempts: u32,
    #[serde(default = "default_retry_backoff_base_ms")]
    pub retry_backoff_base_ms: u64,
}

fn default_grpc_port() -> u16 {
    9090
}

fn default_clickhouse_url() -> String {
    "http://localhost:8123".to_string()
}

fn default_database() -> String {
    "sp_analytics".to_string()
}

fn default_wal_path() -> String {
    "./data/analytics.wal".to_string()
}

fn default_wal_enabled() -> bool {
    true
}

fn default_batch_size() -> usize {
    1000
}

fn default_flush_interval_ms() -> u64 {
    5000
}

fn default_max_retry_attempts() -> u32 {
    10
}

fn default_retry_backoff_base_ms() -> u64 {
    1000
}

impl Config {
    pub fn load() -> Result<Self, figment::Error> {
        let figment = Figment::new()
            .merge(Toml::file("config.toml"))
            .merge(Env::prefixed("SP_ANALYTICS_").split("__"));

        tracing::debug!("figment profile: {:?}", figment.profile());

        let config: Config = figment.extract()?;

        tracing::debug!(
            "loaded config: clickhouse.username = '{}'",
            config.clickhouse.username
        );

        Ok(config)
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: ServerConfig {
                grpc_port: default_grpc_port(),
            },
            clickhouse: ClickHouseConfig {
                url: default_clickhouse_url(),
                database: default_database(),
                username: String::new(),
                password: String::new(),
            },
            wal: WalConfig {
                path: default_wal_path(),
                enabled: default_wal_enabled(),
            },
            ingestion: IngestionConfig {
                batch_size: default_batch_size(),
                flush_interval_ms: default_flush_interval_ms(),
                max_retry_attempts: default_max_retry_attempts(),
                retry_backoff_base_ms: default_retry_backoff_base_ms(),
            },
        }
    }
}
