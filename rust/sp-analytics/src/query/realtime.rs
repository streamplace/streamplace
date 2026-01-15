use anyhow::Result;
use clickhouse::{Client, Row};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Row)]
pub struct StreamerRealtimeRow {
    pub streamer_did: String,
    pub current_viewers: u64,
    pub total_watch_time_ms: u64,
}

pub async fn get_realtime_stats(
    client: &Client,
    streamer_did: Option<&str>,
    window_minutes: u32,
) -> Result<Vec<StreamerRealtimeRow>> {
    let mut query = String::from(
        "SELECT
            streamer_did,
            uniqExact(device_id) as current_viewers,
            sum(
                CASE
                    WHEN session_duration > 0 THEN session_duration
                    ELSE 5000
                END
            ) as total_watch_time_ms
        FROM (
            SELECT
                streamer_did,
                device_id,
                session_id,
                dateDiff('millisecond', min(timestamp), max(timestamp)) as session_duration
            FROM events
            WHERE event_type = 'aq-played'
                AND timestamp > now() - INTERVAL ? MINUTE",
    );

    if streamer_did.is_some() {
        query.push_str(" AND streamer_did = ?");
    }

    query.push_str(
        "
            GROUP BY streamer_did, device_id, session_id
        )
        GROUP BY streamer_did",
    );

    let mut q = client.query(&query).bind(window_minutes);

    if let Some(did) = streamer_did {
        q = q.bind(did);
    }

    let stats = q.fetch_all().await?;
    Ok(stats)
}
