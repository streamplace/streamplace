-- Streamers dimension table
CREATE TABLE IF NOT EXISTS streamers (
    streamer_did String,
    username String,
    display_name String,
    created_at DateTime64(3),
    updated_at DateTime64(3),

    -- aggregates
    total_streams UInt32 DEFAULT 0,
    total_events UInt64 DEFAULT 0,
    follower_count UInt32 DEFAULT 0
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY streamer_did;

-- Follows table
CREATE TABLE IF NOT EXISTS follows (
    follower_did String,
    streamer_did String,
    followed_at DateTime64(3),
    unfollowed_at Nullable(DateTime64(3)),
    status LowCardinality(String),  -- active, unfollowed
    updated_at DateTime64(3)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (follower_did, streamer_did);

-- Streams dimension table
CREATE TABLE IF NOT EXISTS streams (
    stream_id String,
    streamer_did String,
    title String,
    started_at DateTime64(3),
    ended_at Nullable(DateTime64(3)),
    status LowCardinality(String),  -- live, ended, error

    -- metadata
    created_at DateTime64(3),
    updated_at DateTime64(3),

    -- aggregates
    total_viewers UInt32 DEFAULT 0,
    peak_viewers UInt32 DEFAULT 0,
    total_events UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY stream_id;

-- Events table
CREATE TABLE IF NOT EXISTS events (
    event_id UUID,
    event_type LowCardinality(String),
    device_id String,
    did Nullable(String),
    session_id String,

    timestamp DateTime64(3),

    streamer_did String,
    stream_id Nullable(String),

    properties String,
    schema_version UInt16,

    client_version String,
    platform LowCardinality(String)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (streamer_did, timestamp, device_id);

-- Deletion requests audit table
CREATE TABLE IF NOT EXISTS deletion_requests (
    request_id UUID,
    did String,
    requested_at DateTime64(3),
    completed_at Nullable(DateTime64(3)),
    status LowCardinality(String)
)
ENGINE = MergeTree()
ORDER BY requested_at;
