use crate::db::clickhouse::EventRow;
use anyhow::{Context, Result};
use redb::{Database, ReadableTable, ReadableTableMetadata, TableDefinition};
use std::path::Path;
use std::sync::Arc;
use tracing::{debug, info};
use uuid::Uuid;

const EVENTS_TABLE: TableDefinition<&str, &[u8]> = TableDefinition::new("events");

#[derive(Clone)]
pub struct WriteAheadLog {
    db: Arc<Database>,
}

impl WriteAheadLog {
    pub fn new<P: AsRef<Path>>(path: P) -> Result<Self> {
        let db = Database::create(path).context("failed to create WAL database")?;
        Ok(Self { db: Arc::new(db) })
    }

    pub fn write_events(&self, events: &[EventRow]) -> Result<()> {
        let write_txn = self.db.begin_write()?;
        {
            let mut table = write_txn.open_table(EVENTS_TABLE)?;
            for event in events {
                let key = event.event_id.to_string();
                let value = serde_json::to_vec(event)?;
                table.insert(key.as_str(), value.as_slice())?;
            }
        }
        write_txn.commit()?;
        debug!("wrote {} events to WAL", events.len());
        Ok(())
    }

    pub fn remove_events(&self, event_ids: &[Uuid]) -> Result<()> {
        let write_txn = self.db.begin_write()?;
        {
            let mut table = write_txn.open_table(EVENTS_TABLE)?;
            for event_id in event_ids {
                let key = event_id.to_string();
                table.remove(key.as_str())?;
            }
        }
        write_txn.commit()?;
        debug!("removed {} events from WAL", event_ids.len());
        Ok(())
    }

    pub fn read_all_events(&self) -> Result<Vec<EventRow>> {
        let read_txn = self.db.begin_read()?;
        let table = read_txn.open_table(EVENTS_TABLE)?;

        let mut events = Vec::new();
        for entry in table.iter()? {
            let (_, value) = entry?;
            let event: EventRow = serde_json::from_slice(value.value())?;
            events.push(event);
        }

        info!("read {} events from WAL", events.len());
        Ok(events)
    }

    pub fn count(&self) -> Result<usize> {
        let read_txn = self.db.begin_read()?;
        let table = read_txn.open_table(EVENTS_TABLE)?;
        Ok(table.len()? as usize)
    }

    pub fn clear_all(&self) -> Result<()> {
        let write_txn = self.db.begin_write()?;
        {
            let mut table = write_txn.open_table(EVENTS_TABLE)?;
            let keys: Vec<String> = table
                .iter()?
                .map(|entry| entry.map(|(k, _)| k.value().to_string()))
                .collect::<Result<_, _>>()?;

            for key in keys {
                table.remove(key.as_str())?;
            }
        }
        write_txn.commit()?;
        info!("cleared all events from WAL");
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::Utc;
    use tempfile::tempdir;

    #[test]
    fn test_wal_write_and_read() {
        let dir = tempdir().unwrap();
        let wal_path = dir.path().join("test.wal");
        let wal = WriteAheadLog::new(&wal_path).unwrap();

        let events = vec![EventRow {
            event_id: Uuid::new_v4(),
            event_type: "watch".to_string(),
            device_id: "device123".to_string(),
            did: Some("did:plc:user123".to_string()),
            session_id: "session456".to_string(),
            timestamp: Utc::now(),
            streamer_did: "did:plc:streamer789".to_string(),
            stream_id: Some("stream101".to_string()),
            properties: r#"{"duration_ms": 30000}"#.to_string(),
            schema_version: 1,
            client_version: "1.0.0".to_string(),
            platform: "ios".to_string(),
        }];

        wal.write_events(&events).unwrap();
        assert_eq!(wal.count().unwrap(), 1);

        let read_events = wal.read_all_events().unwrap();
        assert_eq!(read_events.len(), 1);
        assert_eq!(read_events[0].event_id, events[0].event_id);
    }

    #[test]
    fn test_wal_remove() {
        let dir = tempdir().unwrap();
        let wal_path = dir.path().join("test.wal");
        let wal = WriteAheadLog::new(&wal_path).unwrap();

        let event_id = Uuid::new_v4();
        let events = vec![EventRow {
            event_id,
            event_type: "watch".to_string(),
            device_id: "device123".to_string(),
            did: None,
            session_id: "session456".to_string(),
            timestamp: Utc::now(),
            streamer_did: "did:plc:streamer789".to_string(),
            stream_id: None,
            properties: "{}".to_string(),
            schema_version: 1,
            client_version: "1.0.0".to_string(),
            platform: "web".to_string(),
        }];

        wal.write_events(&events).unwrap();
        assert_eq!(wal.count().unwrap(), 1);

        wal.remove_events(&[event_id]).unwrap();
        assert_eq!(wal.count().unwrap(), 0);
    }
}
