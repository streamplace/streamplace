use crate::db::clickhouse::{ClickHouseClient, EventRow};
use crate::ingest::{EventBuffer, validate_event};
use crate::proto::analytics_server::Analytics;
use crate::proto::*;
use crate::query::{get_realtime_stats, get_streamer_stats, get_viewer_history};
use chrono::{DateTime, TimeZone, Utc};
use std::str::FromStr;
use std::sync::Arc;
use tonic::{Request, Response, Status};
use tracing::{error, warn};
use uuid::Uuid;

pub struct AnalyticsService {
    clickhouse: ClickHouseClient,
    buffer: Arc<EventBuffer>,
}

impl AnalyticsService {
    pub fn new(clickhouse: ClickHouseClient, buffer: Arc<EventBuffer>) -> Self {
        Self { clickhouse, buffer }
    }
}

#[tonic::async_trait]
impl Analytics for AnalyticsService {
    async fn ingest_events(
        &self,
        request: Request<IngestEventsRequest>,
    ) -> Result<Response<IngestEventsResponse>, Status> {
        let req = request.into_inner();
        let mut accepted = 0u32;
        let mut rejected = 0u32;
        let mut errors = Vec::new();

        let mut valid_events = Vec::new();

        for event in req.events {
            match validate_event(&event) {
                Ok(_) => match convert_to_event_row(&event) {
                    Ok(row) => {
                        valid_events.push(row);
                        accepted += 1;
                    }
                    Err(e) => {
                        rejected += 1;
                        errors.push(format!(
                            "conversion error for event {}: {}",
                            event.event_id, e
                        ));
                    }
                },
                Err(e) => {
                    rejected += 1;
                    errors.push(format!(
                        "validation error for event {}: {}",
                        event.event_id, e
                    ));
                }
            }
        }

        if !valid_events.is_empty() {
            if let Err(e) = self.buffer.add_events(valid_events).await {
                error!("failed to buffer events: {}", e);
                return Err(Status::internal("failed to buffer events"));
            }
        }

        Ok(Response::new(IngestEventsResponse {
            accepted,
            rejected,
            errors,
        }))
    }

    async fn get_streamer_stats(
        &self,
        request: Request<StreamerStatsRequest>,
    ) -> Result<Response<StreamerStatsResponse>, Status> {
        let req = request.into_inner();

        let (total_views, total_watch_time_ms, unique_viewers, daily_stats) = get_streamer_stats(
            self.clickhouse.client(),
            &req.streamer_did,
            req.start_time_ms,
            req.end_time_ms,
        )
        .await
        .map_err(|e| {
            error!("failed to get streamer stats: {}", e);
            Status::internal("query failed")
        })?;

        let daily_stats_proto: Vec<DailyStats> = daily_stats
            .into_iter()
            .map(|s| DailyStats {
                date: s.date,
                views: s.views,
                watch_time_ms: s.watch_time_ms,
                unique_viewers: s.unique_viewers,
            })
            .collect();

        Ok(Response::new(StreamerStatsResponse {
            streamer_did: req.streamer_did,
            total_views,
            total_watch_time_ms,
            unique_viewers,
            daily_stats: daily_stats_proto,
        }))
    }

    async fn get_viewer_history(
        &self,
        request: Request<ViewerHistoryRequest>,
    ) -> Result<Response<ViewerHistoryResponse>, Status> {
        let req = request.into_inner();

        let sessions = get_viewer_history(
            self.clickhouse.client(),
            &req.did,
            req.start_time_ms,
            req.end_time_ms,
            req.limit,
        )
        .await
        .map_err(|e| {
            error!("failed to get viewer history: {}", e);
            Status::internal("query failed")
        })?;

        let sessions_proto: Vec<WatchSession> = sessions
            .into_iter()
            .map(|s| WatchSession {
                session_id: s.session_id,
                streamer_did: s.streamer_did,
                stream_id: s.stream_id,
                start_time_ms: s.start_time_ms,
                end_time_ms: s.end_time_ms,
                duration_ms: s.duration_ms,
            })
            .collect();

        Ok(Response::new(ViewerHistoryResponse {
            sessions: sessions_proto,
        }))
    }

    async fn get_realtime_stats(
        &self,
        request: Request<RealtimeStatsRequest>,
    ) -> Result<Response<RealtimeStatsResponse>, Status> {
        let req = request.into_inner();

        let stats = get_realtime_stats(
            self.clickhouse.client(),
            req.streamer_did.as_deref(),
            req.window_minutes,
        )
        .await
        .map_err(|e| {
            error!("failed to get realtime stats: {}", e);
            Status::internal("query failed")
        })?;

        let streamers: Vec<StreamerRealtimeStats> = stats
            .into_iter()
            .map(|s| StreamerRealtimeStats {
                streamer_did: s.streamer_did,
                current_viewers: s.current_viewers,
                total_watch_time_ms: s.total_watch_time_ms,
            })
            .collect();

        Ok(Response::new(RealtimeStatsResponse { streamers }))
    }

    async fn delete_user_data(
        &self,
        request: Request<DeleteUserDataRequest>,
    ) -> Result<Response<DeleteUserDataResponse>, Status> {
        let req = request.into_inner();

        let request_id = crate::privacy::delete_user_data(&self.clickhouse, req.did)
            .await
            .map_err(|e| {
                error!("failed to create deletion request: {}", e);
                Status::internal("deletion request failed")
            })?;

        Ok(Response::new(DeleteUserDataResponse {
            request_id: request_id.to_string(),
            status: "pending".to_string(),
        }))
    }

    async fn get_deletion_status(
        &self,
        request: Request<GetDeletionStatusRequest>,
    ) -> Result<Response<DeletionStatusResponse>, Status> {
        let req = request.into_inner();

        let request_id = Uuid::from_str(&req.request_id).map_err(|e| {
            warn!("invalid request_id: {}", e);
            Status::invalid_argument("invalid request_id")
        })?;

        let status = crate::privacy::get_deletion_status(&self.clickhouse, request_id)
            .await
            .map_err(|e| {
                error!("failed to get deletion status: {}", e);
                Status::internal("query failed")
            })?;

        match status {
            Some(s) => Ok(Response::new(DeletionStatusResponse {
                request_id: s.request_id.to_string(),
                did: s.did,
                status: s.status,
                requested_at_ms: Some(s.requested_at.timestamp_millis()),
                completed_at_ms: s.completed_at.map(|dt| dt.timestamp_millis()),
            })),
            None => Err(Status::not_found("deletion request not found")),
        }
    }
}

fn convert_to_event_row(event: &Event) -> Result<EventRow, String> {
    let event_id =
        Uuid::from_str(&event.event_id).map_err(|e| format!("invalid event_id: {}", e))?;

    let timestamp_secs = event.timestamp_ms / 1000;
    let timestamp_nanos = ((event.timestamp_ms % 1000) * 1_000_000) as u32;

    let timestamp: DateTime<Utc> = Utc
        .timestamp_opt(timestamp_secs, timestamp_nanos)
        .single()
        .ok_or_else(|| "invalid timestamp".to_string())?;

    Ok(EventRow {
        event_id,
        event_type: event.event_type.clone(),
        device_id: event.device_id.clone(),
        did: event.did.clone(),
        session_id: event.session_id.clone(),
        timestamp,
        streamer_did: event.streamer_did.clone(),
        stream_id: event.stream_id.clone(),
        properties: event.properties_json.clone(),
        schema_version: event.schema_version as u16,
        client_version: event.client_version.clone(),
        platform: event.platform.clone(),
    })
}
