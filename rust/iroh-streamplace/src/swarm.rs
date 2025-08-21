use crate::api::PeerInfo;
use crate::utils::NodeAddr;

/// A peer in a streamplace swarm
#[derive(uniffi::Object, Debug)]
pub struct Peer {
    pub node_addr: NodeAddr,
    pub subscriptions: Vec<String>,
}

impl From<PeerInfo> for Peer {
    fn from(value: PeerInfo) -> Self {
        Self {
            node_addr: NodeAddr::new(&value.node_id.into(), None, vec![]),
            subscriptions: value.subscriptions.into_iter().collect::<_>(),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeSet;
    use std::sync::Arc;
    use std::time::Duration;

    use super::*;
    use crate::endpoint::Endpoint;
    use crate::error::Error;
    use crate::key::PublicKey;
    use crate::receiver::*;
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

    fn peer_set(peers: Vec<Arc<Peer>>) -> BTreeSet<PublicKey> {
        peers
            .into_iter()
            .map(|peer| peer.node_addr.node_id())
            .collect::<BTreeSet<_>>()
    }

    #[tokio::test]
    async fn test_swarm_membership_maintenance() {
        tracing_subscriber::fmt()
            .with_env_filter(
                tracing_subscriber::EnvFilter::from_default_env()
                    .add_directive("iroh_streamplace=debug".parse().unwrap()),
            )
            .init();

        let (no_op_s, _) = tokio::sync::mpsc::channel(5);
        let anchor = new_test_receiver(no_op_s, vec![]).await.unwrap();
        let anchor_addr = anchor.node_addr().await;
        println!("anchor addr: {:?}", anchor_addr.node_id().fmt_short());

        let ep1 = Endpoint::new().await.unwrap();
        let sender = Sender::new(&ep1, vec![Arc::new(anchor_addr.node_id())])
            .await
            .unwrap();

        let (s, _) = tokio::sync::mpsc::channel(5);
        let receiver = new_test_receiver(s, vec![(&anchor_addr.node_id()).into()])
            .await
            .unwrap();

        let sender_addr = sender.node_addr().await;
        println!("sender addr: {:?}", sender_addr.node_id().fmt_short());

        let receiver_addr = receiver.node_addr().await;
        println!("recv addr: {:?}", receiver_addr.node_id().fmt_short());

        let mut i = 0;
        loop {
            let peers = anchor.peers().await.unwrap();
            let peers = peer_set(peers);
            if !peers.is_empty() {
                let peers = peers
                    .into_iter()
                    .map(|k| k.fmt_short())
                    .collect::<Vec<_>>()
                    .join(", ");
                println!(
                    "anchor {} found peers after {} checks: {:?}",
                    anchor_addr.node_id().fmt_short(),
                    i,
                    peers
                );
                break;
            }
            tokio::time::sleep(Duration::from_millis(100)).await;
            i += 1;
            if i > 10 {
                panic!("no peers found after 10 checks");
            }
        }

        let mut i = 0;
        loop {
            let peers = receiver.peers().await.unwrap();
            if peers.len() == 2 {
                let peers = peer_set(peers);
                let peers = peers
                    .into_iter()
                    .map(|k| k.fmt_short())
                    .collect::<Vec<_>>()
                    .join(", ");
                println!(
                    "receiver {} found peers after {} checks: {:?}",
                    receiver_addr.node_id().fmt_short(),
                    i,
                    peers
                );
                break;
            }
            tokio::time::sleep(Duration::from_millis(100)).await;
            i += 1;
            if i > 50 {
                panic!("no peers found after 50 checks");
            }
        }

        // let mut i = 0;
        // loop {
        //     let peers = sender.peers().await.unwrap();
        //     println!("peers: {:?}", peers);
        //     if peers.len() == 1 {
        //         break;
        //     }
        //     tokio::time::sleep(Duration::from_millis(100)).await;
        //     i += 1;
        //     if i > 10 {
        //         panic!("no peers found after 10 checks");
        //     }
        // }
    }
}
