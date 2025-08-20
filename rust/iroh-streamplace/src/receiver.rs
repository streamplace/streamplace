use std::sync::Arc;

use iroh::protocol::Router;

use crate::api::Api;
use crate::endpoint::Endpoint;
use crate::error::Error;
use crate::key::PublicKey;
use crate::utils::{NodeAddr, Peer};

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
        let api = Api::spawn_with_handler(&endpoint.endpoint, anchor_peers, move |id, data| {
            let handler = handler.clone();
            Box::pin(async move {
                handler.handle_data(id, data).await;
            })
        });
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

    #[uniffi::method(async_runtime = "tokio")]
    pub async fn node_addr(&self) -> NodeAddr {
        self.endpoint.node_addr().await
    }
}

#[uniffi::export(with_foreign)]
#[async_trait::async_trait]
pub trait DataHandler: Send + Sync {
    async fn handle_data(&self, topic: String, data: Vec<u8>);
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

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
        tracing_subscriber::fmt()
            .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
            .init();

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

    #[tokio::test]
    async fn test_swarm_membership_maintenance() {
        let (no_op_s, _) = tokio::sync::mpsc::channel(5);
        let bootstrap = new_test_receiver(no_op_s, vec![]).await.unwrap();
        let bootstrap_addr = bootstrap.node_addr().await;
        println!("bootstrap addr: {:?}", bootstrap_addr);

        let ep1 = Endpoint::new().await.unwrap();
        let sender = Sender::new(&ep1, vec![Arc::new(bootstrap_addr.node_id())])
            .await
            .unwrap();

        let (s, _) = tokio::sync::mpsc::channel(5);
        let receiver = new_test_receiver(s, vec![(&bootstrap_addr.node_id()).into()])
            .await
            .unwrap();

        let sender_addr = sender.node_addr().await;
        println!("sender addr: {:?}", sender_addr);

        let receiver_addr = receiver.node_addr().await;
        println!("recv addr: {:?}", receiver_addr);

        let mut i = 0;
        loop {
            let peers = bootstrap.peers().await.unwrap();
            println!("peers: {:?}", peers);
            if !peers.is_empty() {
                break;
            }
            tokio::time::sleep(Duration::from_millis(100)).await;
            i += 1;
            if i > 10 {
                panic!("no peers found after 10 checks");
            }
        }
    }
}
