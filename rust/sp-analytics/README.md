# sp-analytics

rust-based user data collection service for streamplace, storing analytics
events in clickhouse.

## architecture

- **internal gRPC service** - receives events from Go API
- **ClickHouse** - columnar database for analytics storage
- **Write-ahead log (WAL)** - durability layer using redb
- **batched ingestion** - buffers events before flushing to ClickHouse

## quick start

### using docker compose

```bash
cd rust/sp-analytics
docker-compose up
```

this starts both ClickHouse and the analytics service.

### local development

1. start ClickHouse:

```bash
docker run -d -p 8123:8123 -p 9000:9000 \
  --name clickhouse \
  clickhouse/clickhouse-server
```

2. run the service:

```bash
cargo run
```

## configuration

configuration via `config.toml` or environment variables prefixed with
`SP_ANALYTICS_`:

```bash
export SP_ANALYTICS_CLICKHOUSE__URL=http://localhost:8123
export SP_ANALYTICS_CLICKHOUSE__DATABASE=sp_analytics
export SP_ANALYTICS_WAL__ENABLED=true
```

## gRPC API

### IngestEvents

ingest a batch of analytics events.

### GetStreamerStats

retrieve aggregated statistics for a streamer.

### GetViewerHistory

retrieve watch history for a user (by DID).

### GetRealtimeStats

retrieve real-time viewer counts and watch time.

### DeleteUserData

initiate GDPR deletion request for a user's data.

### GetDeletionStatus

check status of a deletion request.

## metrics

prometheus metrics exposed on port 9091:

- `events_ingested_total` - count of events ingested
- `events_rejected_total` - count of events rejected (validation)
- `events_flushed_total` - count of events flushed to ClickHouse
- `flush_duration_ms` - histogram of flush durations
- `wal_replay_total` - count of events replayed from WAL

## testing

```bash
cargo test
```

## integration with Go API

add gRPC client in `pkg/spxrpc/server.go`:

```go
analyticsClient, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
```

forward XRPC events to the analytics service in your handlers.
