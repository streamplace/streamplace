use anyhow::Result;
use clickhouse::{Client, Row};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Row)]
pub struct DailyStatsRow {
    pub date: String,
    pub views: u64,
    pub watch_time_ms: u64,
    pub unique_viewers: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Row)]
pub struct WatchSessionRow {
    pub session_id: String,
    pub streamer_did: String,
    pub stream_id: Option<String>,
    pub start_time_ms: i64,
    pub end_time_ms: i64,
    pub duration_ms: u64,
}

pub async fn get_streamer_stats(
    client: &Client,
    streamer_did: &str,
    start_time_ms: Option<i64>,
    end_time_ms: Option<i64>,
) -> Result<(u64, u64, u64, Vec<DailyStatsRow>)> {
    let mut query = String::from(
        "SELECT
            count() as views,
            sum(JSONExtractInt(properties, 'duration_ms')) as watch_time_ms,
            uniqExact(device_id) as unique_viewers
        FROM events
        WHERE streamer_did = ? AND event_type = 'watch'"
    );

    if start_time_ms.is_some() {
        query.push_str(" AND timestamp >= toDateTime64(?, 3)");
    }
    if end_time_ms.is_some() {
        query.push_str(" AND timestamp <= toDateTime64(?, 3)");
    }

    #[derive(Debug, Row, Deserialize)]
    struct TotalStats {
        views: u64,
        watch_time_ms: u64,
        unique_viewers: u64,
    }

    let mut q = client.query(&query).bind(streamer_did);
    if let Some(start) = start_time_ms {
        q = q.bind(start / 1000);
    }
    if let Some(end) = end_time_ms {
        q = q.bind(end / 1000);
    }

    let totals: TotalStats = q.fetch_one().await?;

    let mut daily_query = String::from(
        "SELECT
            toDate(timestamp) as date,
            count() as views,
            sum(JSONExtractInt(properties, 'duration_ms')) as watch_time_ms,
            uniqExact(device_id) as unique_viewers
        FROM events
        WHERE streamer_did = ? AND event_type = 'watch'"
    );

    if start_time_ms.is_some() {
        daily_query.push_str(" AND timestamp >= toDateTime64(?, 3)");
    }
    if end_time_ms.is_some() {
        daily_query.push_str(" AND timestamp <= toDateTime64(?, 3)");
    }
    daily_query.push_str(" GROUP BY date ORDER BY date DESC");

    let mut dq = client.query(&daily_query).bind(streamer_did);
    if let Some(start) = start_time_ms {
        dq = dq.bind(start / 1000);
    }
    if let Some(end) = end_time_ms {
        dq = dq.bind(end / 1000);
    }

    let daily_stats: Vec<DailyStatsRow> = dq.fetch_all().await?;

    Ok((
        totals.views,
        totals.watch_time_ms,
        totals.unique_viewers,
        daily_stats,
    ))
}

pub async fn get_viewer_history(
    client: &Client,
    did: &str,
    start_time_ms: Option<i64>,
    end_time_ms: Option<i64>,
    limit: u32,
) -> Result<Vec<WatchSessionRow>> {
    let mut query = String::from(
        "SELECT
            session_id,
            streamer_did,
            stream_id,
            toUnixTimestamp64Milli(min(timestamp)) as start_time_ms,
            toUnixTimestamp64Milli(max(timestamp)) as end_time_ms,
            sum(JSONExtractInt(properties, 'duration_ms')) as duration_ms
        FROM events
        WHERE did = ? AND event_type = 'watch'"
    );

    if start_time_ms.is_some() {
        query.push_str(" AND timestamp >= toDateTime64(?, 3)");
    }
    if end_time_ms.is_some() {
        query.push_str(" AND timestamp <= toDateTime64(?, 3)");
    }

    query.push_str(" GROUP BY session_id, streamer_did, stream_id");
    query.push_str(" ORDER BY start_time_ms DESC");
    query.push_str(" LIMIT ?");

    let mut q = client.query(&query).bind(did);
    if let Some(start) = start_time_ms {
        q = q.bind(start / 1000);
    }
    if let Some(end) = end_time_ms {
        q = q.bind(end / 1000);
    }
    q = q.bind(limit);

    let sessions = q.fetch_all().await?;
    Ok(sessions)
}
