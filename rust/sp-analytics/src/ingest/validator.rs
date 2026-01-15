use crate::proto::Event;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ValidationError {
    #[error("missing required field: {0}")]
    MissingField(String),

    #[error("invalid field value: {field}: {reason}")]
    InvalidField { field: String, reason: String },

    #[error("invalid JSON in properties: {0}")]
    InvalidJson(String),
}

pub fn validate_event(event: &Event) -> Result<(), ValidationError> {
    if event.event_id.is_empty() {
        return Err(ValidationError::MissingField("event_id".to_string()));
    }

    if event.event_type.is_empty() {
        return Err(ValidationError::MissingField("event_type".to_string()));
    }

    if event.device_id.is_empty() {
        return Err(ValidationError::MissingField("device_id".to_string()));
    }

    if event.session_id.is_empty() {
        return Err(ValidationError::MissingField("session_id".to_string()));
    }

    if event.streamer_did.is_empty() {
        return Err(ValidationError::MissingField("streamer_did".to_string()));
    }

    if event.client_version.is_empty() {
        return Err(ValidationError::MissingField("client_version".to_string()));
    }

    if event.platform.is_empty() {
        return Err(ValidationError::MissingField("platform".to_string()));
    }

    if event.timestamp_ms <= 0 {
        return Err(ValidationError::InvalidField {
            field: "timestamp_ms".to_string(),
            reason: "must be positive".to_string(),
        });
    }

    if !event.properties_json.is_empty() {
        serde_json::from_str::<serde_json::Value>(&event.properties_json)
            .map_err(|e| ValidationError::InvalidJson(e.to_string()))?;
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn valid_event() -> Event {
        Event {
            event_id: "550e8400-e29b-41d4-a716-446655440000".to_string(),
            event_type: "watch".to_string(),
            device_id: "device123".to_string(),
            did: Some("did:plc:user123".to_string()),
            session_id: "session456".to_string(),
            timestamp_ms: 1704067200000,
            streamer_did: "did:plc:streamer789".to_string(),
            stream_id: Some("stream101".to_string()),
            properties_json: r#"{"duration_ms": 30000}"#.to_string(),
            schema_version: 1,
            client_version: "1.0.0".to_string(),
            platform: "ios".to_string(),
        }
    }

    #[test]
    fn test_valid_event() {
        let event = valid_event();
        assert!(validate_event(&event).is_ok());
    }

    #[test]
    fn test_missing_event_id() {
        let mut event = valid_event();
        event.event_id = String::new();
        assert!(matches!(
            validate_event(&event),
            Err(ValidationError::MissingField(_))
        ));
    }

    #[test]
    fn test_invalid_timestamp() {
        let mut event = valid_event();
        event.timestamp_ms = 0;
        assert!(matches!(
            validate_event(&event),
            Err(ValidationError::InvalidField { .. })
        ));
    }

    #[test]
    fn test_invalid_json() {
        let mut event = valid_event();
        event.properties_json = "not valid json".to_string();
        assert!(matches!(
            validate_event(&event),
            Err(ValidationError::InvalidJson(_))
        ));
    }
}
