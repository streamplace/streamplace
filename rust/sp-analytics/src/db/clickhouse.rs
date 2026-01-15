use crate::config::ClickHouseConfig;
use anyhow::Result;
use chrono::{DateTime, Utc};
use clickhouse::{Client, Row};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Clone)]
pub struct ClickHouseClient {
    client: Client,
}

#[derive(Debug, Clone, Serialize, Deserialize, Row)]
pub struct EventRow {
    #[serde(with = "clickhouse::serde::uuid")]
    pub event_id: Uuid,
    pub event_type: String,
    pub device_id: String,
    pub did: Option<String>,
    pub session_id: String,
    #[serde(with = "clickhouse::serde::chrono::datetime64::millis")]
    pub timestamp: DateTime<Utc>,
    pub streamer_did: String,
    pub stream_id: Option<String>,
    pub properties: String,
    pub schema_version: u16,
    pub client_version: String,
    pub platform: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Row)]
pub struct DeletionRequest {
    pub request_id: Uuid,
    pub did: String,
    pub requested_at: DateTime<Utc>,
    pub completed_at: Option<DateTime<Utc>>,
    pub status: String,
}

impl ClickHouseClient {
    pub fn new(config: &ClickHouseConfig) -> Result<Self> {
        tracing::debug!(
            "creating clickhouse client: url={}, db={}, username='{}', password_len={}",
            config.url,
            config.database,
            config.username,
            config.password.len()
        );

        let client = Client::default()
            .with_url(&config.url)
            .with_database(&config.database);

        let client = if !config.username.is_empty() {
            tracing::debug!("setting username and password");
            client
                .with_user(&config.username)
                .with_password(&config.password)
        } else {
            tracing::debug!("no username set, using default auth");
            client
        };

        Ok(Self { client })
    }

    pub fn client(&self) -> &Client {
        &self.client
    }

    pub async fn insert_events(&self, events: Vec<EventRow>) -> Result<()> {
        if events.is_empty() {
            return Ok(());
        }

        let mut insert = self.client.insert("events")?;
        for event in events {
            insert.write(&event).await?;
        }
        insert.end().await?;

        Ok(())
    }

    pub async fn create_deletion_request(&self, did: String) -> Result<Uuid> {
        let request_id = Uuid::new_v4();
        let request = DeletionRequest {
            request_id,
            did: did.clone(),
            requested_at: Utc::now(),
            completed_at: None,
            status: "pending".to_string(),
        };

        let mut insert = self.client.insert("deletion_requests")?;
        insert.write(&request).await?;
        insert.end().await?;

        self.client
            .query("ALTER TABLE events DELETE WHERE did = ?")
            .bind(&did)
            .execute()
            .await?;

        Ok(request_id)
    }

    pub async fn get_deletion_status(&self, request_id: Uuid) -> Result<Option<DeletionRequest>> {
        let result = self
            .client
            .query("SELECT * FROM deletion_requests WHERE request_id = ?")
            .bind(request_id)
            .fetch_one::<DeletionRequest>()
            .await;

        match result {
            Ok(req) => Ok(Some(req)),
            Err(e) if e.to_string().contains("not found") => Ok(None),
            Err(e) => Err(e.into()),
        }
    }
}
