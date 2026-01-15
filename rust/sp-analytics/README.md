# sp-analytics

## Quick start

### Using docker compose

```bash
cd rust/sp-analytics
docker-compose up
```

This starts both ClickHouse and builds and serves the analytics service.

### Local development

1. Start ClickHouse:

```bash
# in this folder
docker compose up -d clickhouse
```

2. Run the service:

```bash
cargo run
```

## Configuration

Configuration via `config.toml` or environment variables prefixed with
`SP_ANALYTICS_`. For example:

```bash
SP_ANALYTICS_CLICKHOUSE_URL=http://localhost:8123
SP_ANALYTICS_CLICKHOUSE_DATABASE=sp_analytics
SP_ANALYTICS_CLICKHOUSE_USERNAME=default
SP_ANALYTICS_CLICKHOUSE_PASSWORD=yourpassword
SP_ANALYTICS_SERVER__GRPC_PORT=9090
SP_ANALYTICS_WAL__PATH=./analytics.wal
SP_ANALYTICS_WAL__ENABLED=true

```

## Adding new tables

Use `ch2rs` to get close-enough results for the tables you want. It connects to
your local clickhouse database. for example:

```bash
ch2rs events -u default -d sp_analytics -S \
    --derive Clone \
    --derive Serialize \
    --derive Deserialize \
    -T 'UUID=uuid::Uuid' \
    -T 'DateTime64(3)=chrono::DateTime<chrono::Utc>' \
    -O 'properties=String' \
    -O 'blob=Vec<u8>' \
    -B 'blob' \
    -I 'ignored'
```

## Testing

```bash
# in this folder
cargo test
```
