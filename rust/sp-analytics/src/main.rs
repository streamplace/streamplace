use sp_analytics::{
    api::AnalyticsService,
    config::Config,
    db::{ClickHouseClient, schema::run_migrations},
    ingest::{EventBuffer, WriteAheadLog},
    metrics,
    proto::analytics_server::AnalyticsServer,
};
use std::sync::Arc;
use tonic::transport::Server;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    dotenvy::dotenv().ok();
    // get a env variable just in case
    dbg!(std::env::var("SP_ANALYTICS_CLICKHOUSE__USERNAME").ok());
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "sp_analytics=info,tower_http=debug".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    info!("starting sp-analytics service");

    let config = Config::load().unwrap_or_else(|e| {
        error!("failed to load config: {}, using defaults", e);
        Config::default()
    });

    info!("initializing prometheus metrics on port 9091");
    metrics::init_metrics(9091)?;

    info!("connecting to ClickHouse at {}", config.clickhouse.url);
    let clickhouse = ClickHouseClient::new(&config.clickhouse)?;

    info!("running database migrations");
    run_migrations(clickhouse.client()).await?;

    let wal = if config.wal.enabled {
        info!("initializing WAL at {}", config.wal.path);

        if let Some(parent) = std::path::Path::new(&config.wal.path).parent() {
            std::fs::create_dir_all(parent)?;
        }

        Some(WriteAheadLog::new(&config.wal.path)?)
    } else {
        info!("WAL disabled");
        None
    };

    let buffer = Arc::new(EventBuffer::new(
        clickhouse.clone(),
        wal.clone(),
        config.ingestion.clone(),
    ));

    if config.wal.enabled {
        info!("replaying WAL");
        match buffer.replay_wal().await {
            Ok(count) => {
                info!("replayed {} events from WAL", count);
                metrics::record_wal_replay(count as u64);
            }
            Err(e) => {
                error!("failed to replay WAL: {}", e);
            }
        }
    }

    info!("starting periodic flush");
    buffer.clone().start_periodic_flush();

    let service = AnalyticsService::new(clickhouse, buffer);

    let addr = format!("0.0.0.0:{}", config.server.grpc_port).parse()?;
    info!("gRPC server listening on {}", addr);

    Server::builder()
        .add_service(AnalyticsServer::new(service))
        .serve(addr)
        .await?;

    Ok(())
}
