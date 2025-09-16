use iroh::Watcher;

use crate::error::Error;
use crate::utils::NodeAddr;

#[derive(uniffi::Object, Debug, Clone)]
pub struct Endpoint {
    pub(crate) endpoint: iroh::Endpoint,
}

#[uniffi::export]
impl Endpoint {
    /// Create a new endpoint.
    #[uniffi::constructor(async_runtime = "tokio")]
    pub async fn new() -> Result<Self, Error> {
        let endpoint = iroh::Endpoint::builder()
            .discovery_n0()
            .discovery_local_network()
            .bind()
            .await?;

        Ok(Self { endpoint })
    }

    #[uniffi::method(async_runtime = "tokio")]
    pub async fn node_addr(&self) -> NodeAddr {
        let _ = self.endpoint.home_relay().initialized().await;
        let addr = self.endpoint.node_addr().initialized().await;
        addr.into()
    }
}
