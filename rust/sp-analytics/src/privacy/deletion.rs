use crate::db::clickhouse::ClickHouseClient;
use anyhow::Result;
use uuid::Uuid;

pub async fn delete_user_data(clickhouse: &ClickHouseClient, did: String) -> Result<Uuid> {
    clickhouse.create_deletion_request(did).await
}

pub async fn get_deletion_status(
    clickhouse: &ClickHouseClient,
    request_id: Uuid,
) -> Result<Option<crate::db::clickhouse::DeletionRequest>> {
    clickhouse.get_deletion_status(request_id).await
}
