use std::sync::Arc;

use bytes::Bytes;
use iroh::protocol::Router;

use crate::api::Api;
use crate::endpoint::Endpoint;
use crate::error::Error;
use crate::key::PublicKey;
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
    /// anchor_nodes is a list of nodes that will backstop the network. They're
    /// online more often, functioning as bootstrap nodes to get into a livepeer
    /// network, and serve as a consistent rallying point for other nodes.
    /// it's ok to leave the anchor nodes empty for networks of 1.
    /// unlike other peers, subscription updates are *always* sent, and anchor
    /// nodes are never pruned from the available peers list
    #[uniffi::constructor(async_runtime = "tokio")]
    pub async fn new(
        endpoint: &Endpoint,
        anchor_peers: Vec<Arc<PublicKey>>,
    ) -> Result<Sender, Error> {
        let anchor_peers = anchor_peers
            .into_iter()
            .map(|key| {
                let remote_id: iroh::NodeId = key.as_ref().into();
                remote_id
            })
            .collect::<Vec<iroh::NodeId>>();
        let api = Api::spawn(&endpoint.endpoint, anchor_peers);
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
