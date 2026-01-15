use crate::config::IngestionConfig;
use crate::db::clickhouse::{ClickHouseClient, EventRow};
use crate::ingest::wal::WriteAheadLog;
use anyhow::Result;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::Mutex;
use tokio::time;
use tracing::{debug, error, info, warn};
use uuid::Uuid;

pub struct EventBuffer {
    buffer: Arc<Mutex<Vec<EventRow>>>,
    clickhouse: ClickHouseClient,
    wal: Option<WriteAheadLog>,
    config: IngestionConfig,
}

impl EventBuffer {
    pub fn new(
        clickhouse: ClickHouseClient,
        wal: Option<WriteAheadLog>,
        config: IngestionConfig,
    ) -> Self {
        Self {
            buffer: Arc::new(Mutex::new(Vec::new())),
            clickhouse,
            wal,
            config,
        }
    }

    pub async fn add_events(&self, events: Vec<EventRow>) -> Result<()> {
        if let Some(wal) = &self.wal {
            wal.write_events(&events)?;
        }

        let mut buffer = self.buffer.lock().await;
        buffer.extend(events);

        if buffer.len() >= self.config.batch_size {
            drop(buffer);
            self.flush().await?;
        }

        Ok(())
    }

    pub async fn flush(&self) -> Result<()> {
        let events = {
            let mut buffer = self.buffer.lock().await;
            if buffer.is_empty() {
                return Ok(());
            }
            std::mem::take(&mut *buffer)
        };

        let event_count = events.len();
        debug!("flushing {} events to ClickHouse", event_count);

        match self.flush_with_retry(&events).await {
            Ok(_) => {
                info!("successfully flushed {} events", event_count);

                if let Some(wal) = &self.wal {
                    let event_ids: Vec<Uuid> = events.iter().map(|e| e.event_id).collect();
                    if let Err(e) = wal.remove_events(&event_ids) {
                        error!("failed to remove events from WAL: {}", e);
                    }
                }

                Ok(())
            }
            Err(e) => {
                error!("failed to flush events after retries: {}", e);

                let mut buffer = self.buffer.lock().await;
                buffer.extend(events);

                Err(e)
            }
        }
    }

    async fn flush_with_retry(&self, events: &[EventRow]) -> Result<()> {
        let mut attempts = 0;
        let max_attempts = self.config.max_retry_attempts;

        loop {
            match self.clickhouse.insert_events(events.to_vec()).await {
                Ok(_) => return Ok(()),
                Err(e) => {
                    attempts += 1;
                    if attempts >= max_attempts {
                        return Err(e);
                    }

                    let backoff = Duration::from_millis(
                        self.config.retry_backoff_base_ms * 2u64.pow(attempts - 1),
                    );

                    warn!(
                        "flush attempt {}/{} failed: {}, retrying in {:?}",
                        attempts, max_attempts, e, backoff
                    );

                    time::sleep(backoff).await;
                }
            }
        }
    }

    pub fn start_periodic_flush(self: Arc<Self>) {
        let flush_interval = Duration::from_millis(self.config.flush_interval_ms);

        tokio::spawn(async move {
            let mut interval = time::interval(flush_interval);

            loop {
                interval.tick().await;

                if let Err(e) = self.flush().await {
                    error!("periodic flush failed: {}", e);
                }
            }
        });
    }

    pub async fn replay_wal(&self) -> Result<usize> {
        let wal = match &self.wal {
            Some(wal) => wal,
            None => return Ok(0),
        };

        let events = wal.read_all_events()?;
        let count = events.len();

        if count == 0 {
            return Ok(0);
        }

        info!("replaying {} events from WAL", count);

        dbg!(&events);

        match self.flush_with_retry(&events).await {
            Ok(_) => {
                info!("successfully replayed {} events", count);
                let event_ids: Vec<Uuid> = events.iter().map(|e| e.event_id).collect();
                wal.remove_events(&event_ids)?;
                Ok(count)
            }
            Err(e) => {
                error!("failed to replay WAL events: {}", e);
                Err(e)
            }
        }
    }
}
