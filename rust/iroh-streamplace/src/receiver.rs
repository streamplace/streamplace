use std::sync::Arc;

use iroh::protocol::Router;

use crate::api::{Api, DEFAULT_PEER_INFO_REPUBLISH_INTERVAL, DEFAULT_PEER_PRUNE_INTERVAL};
use crate::endpoint::Endpoint;
use crate::error::Error;
use crate::key::PublicKey;
use crate::swarm::Peer;
use crate::utils::NodeAddr;

#[derive(uniffi::Object)]
pub struct Receiver {
    endpoint: Endpoint,
    api: Api,
    _router: iroh::protocol::Router,
}

#[uniffi::export]
impl Receiver {
    /// Create a new receiver.
    #[uniffi::constructor(async_runtime = "tokio")]
    pub async fn new(
        endpoint: &Endpoint,
        anchor_peers: Vec<Arc<PublicKey>>,
        handler: Arc<dyn DataHandler>,
    ) -> Result<Receiver, Error> {
        let anchor_peers = anchor_peers
            .into_iter()
            .map(|key| {
                let remote_id: iroh::NodeId = key.as_ref().into();
                remote_id
            })
            .collect::<Vec<iroh::NodeId>>();
        let api = Api::spawn_with_opts(
            &endpoint.endpoint,
            anchor_peers,
            move |id, data| {
                let handler = handler.clone();
                Box::pin(async move {
                    handler.handle_data(id, data).await;
                })
            },
            DEFAULT_PEER_INFO_REPUBLISH_INTERVAL,
            DEFAULT_PEER_PRUNE_INTERVAL,
        );
        let router = Router::builder(endpoint.endpoint.clone())
            .accept(Api::ALPN, api.expose())
            .spawn();

        Ok(Receiver {
            endpoint: endpoint.clone(),
            api,
            _router: router,
        })
    }

    /// list all subscriptions the remote knows about
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn peers(&self) -> Result<Vec<Arc<Peer>>, Error> {
        let subs = self.api.peers().await?;
        let mut subs_arc = Vec::new();
        for sub in subs {
            subs_arc.push(Arc::new(sub.into()));
        }
        Ok(subs_arc)
    }

    /// Subscribe to the given topic on the remote.
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn subscribe(&self, remote_id: Arc<PublicKey>, topic: &str) -> Result<(), Error> {
        let remote_id: iroh::NodeId = remote_id.as_ref().into();
        let api = Api::connect(self.endpoint.endpoint.clone(), remote_id);
        api.subscribe(topic.to_string(), self.endpoint.endpoint.node_id())
            .await?;
        Ok(())
    }

    /// Unsubscribe from this topic on the remote.
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn unsubscribe(&self, remote_id: Arc<PublicKey>, topic: &str) -> Result<(), Error> {
        let remote_id: iroh::NodeId = remote_id.as_ref().into();
        let api = Api::connect(self.endpoint.endpoint.clone(), remote_id);
        api.unsubscribe(topic.to_string(), self.endpoint.endpoint.node_id())
            .await?;
        Ok(())
    }

    /// Get our node address
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn node_addr(&self) -> NodeAddr {
        self.endpoint.node_addr().await
    }

    /// tell the network that we're leaving. This should only be called just before disconnecting.
    #[uniffi::method(async_runtime = "tokio")]
    pub async fn leaving(&self) -> Result<(), Error> {
        self.api.leaving().await?;
        Ok(())
    }
}

#[uniffi::export(with_foreign)]
#[async_trait::async_trait]
pub trait DataHandler: Send + Sync {
    async fn handle_data(&self, topic: String, data: Vec<u8>);
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::sender::Sender;
    use iroh::NodeId;

    #[derive(Debug, Clone)]
    struct TestHandler {
        messages: tokio::sync::mpsc::Sender<(String, Vec<u8>)>,
    }

    #[async_trait::async_trait]
    impl DataHandler for TestHandler {
        async fn handle_data(&self, topic: String, data: Vec<u8>) {
            self.messages.send((topic, data)).await.unwrap();
        }
    }

    async fn new_test_receiver(
        handler_msg_sender: tokio::sync::mpsc::Sender<(String, Vec<u8>)>,
        anchors: Vec<NodeId>,
    ) -> Result<Receiver, Error> {
        let ep = Endpoint::new().await.unwrap();
        let handler = TestHandler {
            messages: handler_msg_sender,
        };
        let anchors = anchors
            .iter()
            .map(|anchor| Arc::new(anchor.clone().into()))
            .collect::<Vec<_>>();
        Receiver::new(&ep, anchors, Arc::new(handler.clone())).await
    }

    #[tokio::test]
    async fn test_subscription_roundtrip() {
        // tracing_subscriber::fmt()
        //     .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        //     .init();

        let ep1 = Endpoint::new().await.unwrap();
        let sender = Sender::new(&ep1, vec![]).await.unwrap();

        let (s, mut r) = tokio::sync::mpsc::channel(5);

        let receiver = new_test_receiver(s, vec![]).await.unwrap();

        let sender_addr = sender.node_addr().await;
        println!("sender addr: {:?}", sender_addr);

        let receiver_addr = receiver.node_addr().await;
        println!("recv addr: {:?}", receiver_addr);

        // subscribe
        receiver
            .subscribe(Arc::new(sender_addr.node_id()), "foo")
            .await
            .unwrap();

        // send a few messages
        for i in 0u8..5 {
            sender.send("foo", &[i, 0, 0, 0]).await.unwrap();
        }

        // make sure the receiver got them
        for i in 0u8..5 {
            let (topic, msg) = r.recv().await.unwrap();
            assert_eq!(topic, "foo");
            assert_eq!(msg, vec![i, 0, 0, 0]);
        }

        // unsubscribe
        receiver
            .unsubscribe(Arc::new(sender_addr.node_id()), "foo")
            .await
            .unwrap();

        // send a message, shouldn't error
        sender.send("foo", &[1]).await.unwrap();

        // no message received, times out
        let res = tokio::time::timeout(std::time::Duration::from_millis(200), async {
            r.recv().await.unwrap();
        })
        .await;
        assert!(res.is_err());
    }
}
