FROM rust:1-slim-trixie as builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
  ca-certificates protobuf-compiler \
  && rm -rf /var/lib/apt/lists/*

# copy workspace manifest
COPY Cargo.toml Cargo.lock ./

# copy all workspace members (needed for workspace resolution)
COPY rust/export-c2pa-schema rust/export-c2pa-schema
COPY rust/iroh-streamplace rust/iroh-streamplace
COPY rust/sp-analytics rust/sp-analytics

# build only sp-analytics
RUN cargo build --release -p sp-analytics

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/target/release/sp-analytics /usr/local/bin/sp-analytics

RUN mkdir -p /app/data

ENV SP_ANALYTICS_WAL__PATH=/app/data/analytics.wal

EXPOSE 9090 9091

CMD ["sp-analytics"]
