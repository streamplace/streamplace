//! Protocol API

use std::collections::{BTreeMap, BTreeSet};

use bytes::Bytes;
use iroh::{Endpoint, NodeId, protocol::ProtocolHandler};
use irpc::{
    Client, WithChannels,
    channel::{mpsc, oneshot},
    rpc::RemoteService,
    rpc_requests,
};
use irpc_iroh::{IrohProtocol, IrohRemoteConnection};
use n0_future::future::Boxed;
use serde::{Deserialize, Serialize};
use tracing::{debug, warn};

/// Subscribe to the given `key`
#[derive(Debug, Serialize, Deserialize)]
struct Subscribe {
    key: String,
    // TODO: verify
    remote_id: NodeId,
}

/// Unsubscribe from the given `key`
#[derive(Debug, Serialize, Deserialize)]
struct Unsubscribe {
    key: String,
    // TODO: verify
    remote_id: NodeId,
}

/// Send a segment of video to another peer in a subscription
#[derive(Debug, Serialize, Deserialize)]
struct SendSegment {
    key: String,
    data: Bytes,
}

/// Receive a segment of video from another peer in a subscription
#[derive(Debug, Clone, Serialize, Deserialize)]
struct RecvSegment {
    key: String,
    data: Bytes,
}

/// List all peers, and the subscriptions that they're believed to have
/// "believed", because subscription info can be out of date
#[derive(Debug, Clone, Serialize, Deserialize)]
struct Peers {}

/// Request a node list out it's current subscriptions
#[derive(Debug, Clone, Serialize, Deserialize)]
struct GetSubscriptions {}

/// List subscriptions this node has for given streams
#[derive(Debug, Clone, Ord, PartialOrd, PartialEq, Eq, Serialize, Deserialize)]
pub(crate) struct PeerSubscriptions {
    pub(crate) node_id: NodeId,
    pub(crate) subscriptions: BTreeSet<String>,
}

// Use the macro to generate both the Protocol and Message enums
// plus implement Channels for each type
#[rpc_requests(message = Message)]
#[derive(Serialize, Deserialize, Debug)]
enum Protocol {
    // swarm coordination
    #[rpc(tx=mpsc::Sender<PeerSubscriptions>)]
    Peers(Peers),
    #[rpc(tx=oneshot::Sender<()>)]
    AnnounceSubscriptions(PeerSubscriptions),
    #[rpc(tx=oneshot::Sender<PeerSubscriptions>)]
    GetSubscriptions(GetSubscriptions),

    // stream replication
    #[rpc(tx=oneshot::Sender<()>)]
    Subscribe(Subscribe),
    #[rpc(tx=oneshot::Sender<()>)]
    Unsubscribe(Unsubscribe),
    #[rpc(tx=oneshot::Sender<()>)]
    SendSegment(SendSegment),
    #[rpc(tx=oneshot::Sender<()>)]
    RecvSegment(RecvSegment),
}

/// Actor holds all state necessary to respond to all RPC requests. The actor
/// handles rpc message requests & emits responses
struct Actor {
    endpoint: iroh::Endpoint,
    recv: tokio::sync::mpsc::Receiver<Message>,
    /// set of all peers we believe to be life in the swarm
    peers: BTreeSet<PeerSubscriptions>,
    /// set of stream subscriptions we're receiving data for
    subscriptions: BTreeMap<String, BTreeSet<NodeId>>,
    /// pool of open RPC connections
    connections: BTreeMap<NodeId, Connection>,
    handler: Box<dyn Fn(String, Vec<u8>) -> Boxed<()> + Send + Sync + 'static>,
}

#[derive(Debug)]
struct Connection {
    _id: NodeId,
    rpc: Client<Protocol>,
}

impl Actor {
    fn spawn(
        endpoint: &iroh::Endpoint,
        handler: impl Fn(String, Vec<u8>) -> Boxed<()> + Send + Sync + 'static,
    ) -> Api {
        let (tx, rx) = tokio::sync::mpsc::channel(1);
        let actor = Self {
            endpoint: endpoint.clone(),
            recv: rx,
            peers: BTreeSet::new(),
            subscriptions: BTreeMap::new(),
            connections: BTreeMap::new(),
            handler: Box::new(handler),
        };
        n0_future::task::spawn(actor.run());
        Api {
            inner: Client::local(tx),
        }
    }

    async fn run(mut self) {
        while let Some(msg) = self.recv.recv().await {
            self.handle(msg).await;
        }
    }

    fn subs(&self) -> PeerSubscriptions {
        let mut subscriptions = BTreeSet::new();
        for key in self.subscriptions.keys() {
            subscriptions.insert(key.clone());
        }

        PeerSubscriptions {
            node_id: self.endpoint.node_id(),
            subscriptions,
        }
    }

    async fn handle(&mut self, msg: Message) {
        match msg {
            // stream coordination
            Message::Peers(sub) => {
                debug!("peers {:?}", sub);
                let WithChannels { tx, .. } = sub;

                // stream over the list of peers we know about
                for sub in &self.peers {
                    if tx.send(sub.clone()).await.is_err() {
                        break;
                    }
                }
            }
            Message::AnnounceSubscriptions(sub) => {
                let WithChannels { inner, tx, .. } = sub;
                // update our tracked state about this peer
                self.peers.insert(inner);

                tx.send(()).await.ok();
            }
            Message::GetSubscriptions(sub) => {
                let WithChannels { tx, .. } = sub;
                tx.send(self.subs()).await.ok();
            }

            // stream replication
            Message::Subscribe(sub) => {
                debug!("subscribe {:?}", sub);
                let WithChannels { tx, inner, .. } = sub;

                self.subscriptions
                    .entry(inner.key)
                    .or_default()
                    .insert(inner.remote_id);

                tx.send(()).await.ok();
            }
            Message::Unsubscribe(sub) => {
                debug!("unsubscribe {:?}", sub);
                let WithChannels { tx, inner, .. } = sub;

                if let Some(e) = self.subscriptions.get_mut(&inner.key) {
                    e.remove(&inner.remote_id);
                }

                tx.send(()).await.ok();
            }
            Message::SendSegment(segment) => {
                debug!("send segment {:?}", segment);
                let WithChannels { tx, inner, .. } = segment;

                let msg = RecvSegment {
                    key: inner.key.clone(),
                    data: inner.data.clone(),
                };

                for (key, remotes) in &self.subscriptions {
                    if key == &inner.key {
                        for remote in remotes {
                            debug!("sending to topic {}: {}", key, remote);

                            // ensure connection
                            if !self.connections.contains_key(remote) {
                                let conn = IrohRemoteConnection::new(
                                    self.endpoint.clone(),
                                    (*remote).into(),
                                    Api::ALPN.to_vec(),
                                );

                                let conn = Connection {
                                    rpc: Client::boxed(conn),
                                    _id: *remote,
                                };
                                self.connections.insert(*remote, conn);
                            }
                            let conn = self.connections.get(remote).expect("just checked");

                            if let Err(err) = conn.rpc.rpc(msg.clone()).await {
                                warn!("failed to send to {}: {:?}", remote, err);
                                // remove conn
                                self.connections.remove(remote);
                            }
                        }
                    }
                }

                tx.send(()).await.ok();
            }
            Message::RecvSegment(segment) => {
                debug!("recv segment {:?}", segment);
                let WithChannels { tx, inner, .. } = segment;
                (self.handler)(inner.key, inner.data.to_vec()).await;
                tx.send(()).await.ok();
            }
        }
    }
}

/// The actual API to interact with
pub(crate) struct Api {
    inner: Client<Protocol>,
}

impl Api {
    pub(crate) const ALPN: &[u8] = b"/iroh/streamplace/1";

    pub(crate) fn spawn(endpoint: &iroh::Endpoint) -> Self {
        Actor::spawn(endpoint, |_, _| Box::pin(async move {}))
    }

    pub(crate) fn spawn_with_handler(
        endpoint: &iroh::Endpoint,
        handler: impl Fn(String, Vec<u8>) -> Boxed<()> + Send + Sync + 'static,
    ) -> Self {
        Actor::spawn(endpoint, handler)
    }

    pub(crate) fn connect(endpoint: Endpoint, addr: impl Into<iroh::NodeAddr>) -> Api {
        let conn = IrohRemoteConnection::new(endpoint, addr.into(), Self::ALPN.to_vec());
        Api {
            inner: Client::boxed(conn),
        }
    }

    pub(crate) fn expose(&self) -> impl ProtocolHandler {
        let local = self
            .inner
            .as_local()
            .expect("can not listen on remote service");
        IrohProtocol::new(Protocol::remote_handler(local))
    }

    pub(crate) async fn peers(&self) -> irpc::Result<mpsc::Receiver<PeerSubscriptions>> {
        self.inner.server_streaming(Peers {}, 100).await
    }

    pub(crate) async fn subscribe(&self, key: String, self_id: NodeId) -> irpc::Result<()> {
        self.inner
            .rpc(Subscribe {
                key,
                remote_id: self_id,
            })
            .await
    }

    pub(crate) async fn unsubscribe(&self, key: String, self_id: NodeId) -> irpc::Result<()> {
        self.inner
            .rpc(Unsubscribe {
                key,
                remote_id: self_id,
            })
            .await
    }

    /// Send this segment to all subscriptions.
    pub(crate) async fn send_segment(&self, key: String, data: Bytes) -> irpc::Result<()> {
        let msg = SendSegment { key, data };
        self.inner.rpc(msg).await
    }
}
