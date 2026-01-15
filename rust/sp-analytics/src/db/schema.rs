use anyhow::Result;
use clickhouse::Client;
use tracing::info;

const MIGRATIONS: &[&str] = &[include_str!("../../migrations/001_initial.sql")];

// TODO: better way to do CH migrations than to run them on every startup
pub async fn run_migrations(client: &Client) -> Result<()> {
    info!("running database migrations");

    for (idx, migration) in MIGRATIONS.iter().enumerate() {
        info!("running migration {}", idx + 1);

        for statement in migration.split(';').filter(|s| !s.trim().is_empty()) {
            client.query(statement).execute().await?;
        }
    }

    info!("migrations complete");
    Ok(())
}
