use bytes::Bytes;
use iroh::protocol::Router;

use crate::api::Api;
use crate::endpoint::Endpoint;
use crate::error::Error;
use crate::utils::NodeAddr;

#[derive(uniffi::Object)]
pub struct Sender {
    endpoint: Endpoint,
    api: Api,
    _router: iroh::protocol::Router,
}

#[uniffi::export]
impl Sender {
    /// Create a new sender.
    #[uniffi::constructor(async_runtime = "tokio")]
    pub async fn new(endpoint: &Endpoint) -> Result<Sender, Error> {
        let api = Api::spawn(&endpoint.endpoint);
        let router = Router::builder(endpoint.endpoint.clone())
            .accept(Api::ALPN, api.expose())
            .spawn();

        Ok(Sender {
            endpoint: endpoint.clone(),
            api,
            _router: router,
        })
    }

    /// Sends the given data to all subscribers that have subscribed to this `key`.
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn send(&self, key: &str, data: &[u8]) -> Result<(), Error> {
        self.api
            .send_segment(key.to_string(), Bytes::copy_from_slice(data))
            .await?;
        Ok(())
    }

    #[uniffi::method(async_runtime = "tokio")]
    pub async fn node_addr(&self) -> NodeAddr {
        self.endpoint.node_addr().await
    }
}
