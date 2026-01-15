use metrics_exporter_prometheus::PrometheusBuilder;
use std::net::SocketAddr;
use tracing::info;

pub fn init_metrics(port: u16) -> Result<(), Box<dyn std::error::Error>> {
    let addr: SocketAddr = ([0, 0, 0, 0], port).into();

    PrometheusBuilder::new()
        .with_http_listener(addr)
        .install()?;

    info!("prometheus metrics server listening on {}", addr);
    Ok(())
}

pub fn record_events_ingested(count: u64) {
    metrics::counter!("events_ingested_total").increment(count);
}

pub fn record_events_rejected(count: u64) {
    metrics::counter!("events_rejected_total").increment(count);
}

pub fn record_events_flushed(count: u64) {
    metrics::counter!("events_flushed_total").increment(count);
}

pub fn record_flush_duration(duration_ms: f64) {
    metrics::histogram!("flush_duration_ms").record(duration_ms);
}

pub fn record_wal_replay(count: u64) {
    metrics::counter!("wal_replay_total").increment(count);
}
