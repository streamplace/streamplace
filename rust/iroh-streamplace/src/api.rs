//! Protocol API

use std::{
    collections::{BTreeMap, BTreeSet, HashMap},
    time::Duration,
};

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
use tokio::task::JoinHandle;
use tracing::{debug, warn};

use crate::utils::timestamp;

pub(crate) const DEFAULT_PEER_INFO_REPUBLISH_INTERVAL: Duration = Duration::from_secs(10);
pub(crate) const DEFAULT_PEER_PRUNE_INTERVAL: Duration = Duration::from_secs(60);

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

/// Ask a remote peer to return a stream of it's known PeerInfos
#[derive(Debug, Clone, Serialize, Deserialize)]
struct RequestPeerInfos {
    remote_id: NodeId,
}

/// Return a stream of peer infos
#[derive(Debug, Clone, Serialize, Deserialize)]
struct HandlePeerInfosRequest {}

/// Return this peer's local state
#[derive(Debug, Clone, Serialize, Deserialize)]
struct MyPeerInfo {}

/// List all peers, and the subscriptions that they're believed to have
/// "believed", because subscription info can be out of date
#[derive(Debug, Clone, Serialize, Deserialize)]
struct MyPeers {}

/// Prune peers that haven't been seen since the given timestamp
#[derive(Debug, Clone, Serialize, Deserialize)]
struct PruneMyPeers {
    cutoff_timestamp: u64,
}

/// Inform a remote node about our subscriptions
#[derive(Debug, Clone, Serialize, Deserialize)]
struct SendPeerInfo {
    // the peer receiving the announcement
    remote_id: NodeId,
    // info about the peer being announced
    info: PeerInfo,
}

/// Request a node list out it's current subscriptions
#[derive(Debug, Clone, Serialize, Deserialize)]
struct GetSubscriptions {
    // the peer to get subscriptions from
    node_id: NodeId,
}

/// Tell all remote peers that we're leaving the network
#[derive(Debug, Clone, Serialize, Deserialize)]
struct BroadcastLeaving {}

/// Tell a remote node that we're leaving the network
#[derive(Debug, Clone, Serialize, Deserialize)]
struct HandleLeaving {
    /// the node id of the peer leaving
    node_id: NodeId,
}

/// details about a peer in the network
#[derive(Debug, Clone, Ord, PartialOrd, PartialEq, Eq, Serialize, Deserialize)]
pub(crate) struct PeerInfo {
    pub(crate) node_id: NodeId,
    pub(crate) subscriptions: BTreeSet<String>,
    pub(crate) timestamp: u64,
}

// Use the macro to generate both the Protocol and Message enums
// plus implement Channels for each type
#[rpc_requests(message = Message)]
#[derive(Serialize, Deserialize, Debug)]
enum Protocol {
    // swarm coordination
    #[rpc(tx=mpsc::Sender<PeerInfo>)]
    MyPeers(MyPeers),
    #[rpc(tx=oneshot::Sender<PeerInfo>)]
    MyPeerInfo(MyPeerInfo),
    #[rpc(tx=oneshot::Sender<()>)]
    PruneMyPeers(PruneMyPeers),
    #[rpc(tx=oneshot::Sender<()>)]
    BroadcastLeaving(BroadcastLeaving),
    #[rpc(tx=oneshot::Sender<()>)]
    HandleLeaving(HandleLeaving),

    #[rpc(tx=oneshot::Sender<()>)]
    RequestPeerInfos(RequestPeerInfos),
    #[rpc(tx=mpsc::Sender<PeerInfo>)]
    HandlePeerInfosRequest(HandlePeerInfosRequest),

    #[rpc(tx=oneshot::Sender<()>)]
    SendPeerInfo(SendPeerInfo),
    #[rpc(tx=oneshot::Sender<()>)]
    RecvPeerInfo(PeerInfo),

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
    /// peers we'll permanently broadcast to
    anchor_peers: Vec<NodeId>,
    /// set of all peers we believe to be life in the swarm
    peers: HashMap<NodeId, PeerInfo>,
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
        anchor_peers: Vec<NodeId>,
        handler: impl Fn(String, Vec<u8>) -> Boxed<()> + Send + Sync + 'static,
    ) -> Api {
        let anchors = anchor_peers.clone();
        let (tx, rx) = tokio::sync::mpsc::channel(1);
        let actor = Self {
            endpoint: endpoint.clone(),
            recv: rx,
            anchor_peers,
            peers: HashMap::new(),
            subscriptions: BTreeMap::new(),
            connections: BTreeMap::new(),
            handler: Box::new(handler),
        };

        let info = actor.my_peer_info();
        n0_future::task::spawn(actor.run());
        let client = Client::local(tx);

        // tell anchor peers we exist
        let client2 = client.clone();
        n0_future::task::spawn(async move {
            for anchor in anchors {
                debug!("Announcing subscriptions to anchor peer: {}", anchor);
                client2
                    .rpc(SendPeerInfo {
                        remote_id: anchor,
                        info: info.clone(),
                    })
                    .await
                    .unwrap();
            }
        });

        Api { inner: client }
    }

    async fn run(mut self) {
        while let Some(msg) = self.recv.recv().await {
            self.handle(msg).await;
        }
    }

    fn my_peer_info(&self) -> PeerInfo {
        let mut subscriptions = BTreeSet::new();
        for key in self.subscriptions.keys() {
            subscriptions.insert(key.clone());
        }

        PeerInfo {
            node_id: self.endpoint.node_id(),
            subscriptions,
            timestamp: timestamp(),
        }
    }

    // ensure a connection to exists, re-using from the pool if present
    async fn get_conn(&mut self, remote: &iroh::PublicKey) -> &Connection {
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
        self.connections.get(remote).expect("just checked")
    }

    async fn handle(&mut self, msg: Message) {
        match msg {
            // swarm coordination
            Message::MyPeers(sub) => {
                let WithChannels { tx, .. } = sub;

                // keep track of peers we've already sent
                let mut sent = BTreeSet::new();

                // stream over the list of peers we know about
                for (id, sub) in &self.peers {
                    sent.insert(*id);
                    if tx.send(sub.clone()).await.is_err() {
                        break;
                    }
                }

                // send over any anchor peers we know about, but haven't already
                // sent from our peers list. these go with empty subscription
                // sets, which isn't great.
                for anchor in &self.anchor_peers {
                    if sent.contains(anchor) {
                        continue;
                    }
                    let sub = PeerInfo {
                        node_id: *anchor,
                        subscriptions: BTreeSet::new(),
                        timestamp: timestamp(),
                    };
                    if tx.send(sub).await.is_err() {
                        break;
                    }
                }
            }
            Message::MyPeerInfo(sub) => {
                let WithChannels { tx, .. } = sub;
                tx.send(self.my_peer_info()).await.ok();
            }
            Message::PruneMyPeers(sub) => {
                debug!(
                    message = "PruneMyPeers",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { tx, inner, .. } = sub;
                // prune peers that haven't been seen since the given timestamp
                self.peers
                    .retain(|_, peer| peer.timestamp >= inner.cutoff_timestamp);
                tx.send(()).await.ok();
            }

            Message::RequestPeerInfos(list) => {
                debug!(
                    message = "RequestPeerInfos",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { inner, tx, .. } = list;
                let conn = self.get_conn(&inner.remote_id).await;
                let mut rx = conn
                    .rpc
                    .server_streaming(HandlePeerInfosRequest {}, 1000)
                    .await
                    .unwrap();
                while let Some(mut peer_info) = rx.recv().await.unwrap() {
                    // update our tracked state about this peer, using timestamps
                    // to avoid confusion from external sources
                    peer_info.timestamp = timestamp();
                    self.peers.insert(peer_info.node_id, peer_info);
                }

                tx.send(()).await.ok();
            }
            Message::HandlePeerInfosRequest(list) => {
                debug!(
                    message = "HandlePeerInfosRequest",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { tx, .. } = list;
                for (_, peer) in self.peers.clone() {
                    if let Err(e) = tx.send(peer).await {
                        tracing::error!("send peer error: {:?}", e);
                    }
                }
            }

            Message::SendPeerInfo(info) => {
                let WithChannels { inner, tx, .. } = info;
                debug!(
                    "Received announce subscriptions. me: {:?} them: {:?}",
                    self.endpoint.node_id().fmt_short(),
                    inner.info.node_id.fmt_short()
                );
                let conn = self.get_conn(&inner.remote_id).await;
                // conn.rpc.rpc(inner.info).await?;

                if let Err(err) = conn.rpc.rpc(inner.info).await {
                    warn!("failed to send to {}: {:?}", inner.remote_id, err);
                    // remove conn on failure
                    self.connections.remove(&inner.remote_id);
                }

                tx.send(()).await.ok();
            }
            Message::RecvPeerInfo(sub) => {
                debug!(
                    message = "RecvPeerInfo",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { tx, mut inner, .. } = sub;
                // update our tracked state about this peer, using timestamps
                // to avoid confusion from external sources
                inner.timestamp = timestamp();
                self.peers.insert(inner.node_id, inner);
                tx.send(()).await.ok();
            }
            Message::BroadcastLeaving(leaving) => {
                debug!(
                    message = "BroadcastLeaving",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { tx, .. } = leaving;
                let node_id = self.endpoint.node_id();
                let remotes = self
                    .peers
                    .values()
                    .map(|peer| peer.node_id)
                    .collect::<Vec<_>>();
                for remote_node_id in remotes {
                    // ensure connection
                    let conn = self.get_conn(&remote_node_id).await;
                    if let Err(err) = conn.rpc.rpc(HandleLeaving { node_id }).await {
                        tracing::error!("failed to handle leaving: {}", err);
                    }
                }
                tx.send(()).await.ok();
            }
            Message::HandleLeaving(leaving) => {
                debug!(
                    message = "HandleLeaving",
                    node_id = %self.endpoint.node_id().fmt_short()
                );
                let WithChannels { tx, inner, .. } = leaving;
                self.peers.remove(&inner.node_id);
                tx.send(()).await.ok();
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

                for (key, remotes) in &self.subscriptions.clone() {
                    if key == &inner.key {
                        for remote in remotes {
                            debug!("sending to topic {}: {}", key, remote);

                            // ensure connection
                            let conn = self.get_conn(remote).await;

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
#[derive(Debug, Clone)]
pub(crate) struct Api {
    inner: Client<Protocol>,
}

impl Api {
    pub(crate) const ALPN: &[u8] = b"/iroh/streamplace/1";

    pub(crate) fn spawn(endpoint: &iroh::Endpoint, anchor_peers: Vec<NodeId>) -> Self {
        Api::spawn_with_opts(
            endpoint,
            anchor_peers,
            |_, _| Box::pin(async move {}),
            DEFAULT_PEER_INFO_REPUBLISH_INTERVAL,
            DEFAULT_PEER_PRUNE_INTERVAL,
        )
    }

    pub(crate) fn spawn_with_opts(
        endpoint: &iroh::Endpoint,
        anchor_peers: Vec<NodeId>,
        handler: impl Fn(String, Vec<u8>) -> Boxed<()> + Send + Sync + 'static,
        peer_info_broadcast_interval: Duration,
        peer_prune_interval: Duration,
    ) -> Self {
        let anchors = anchor_peers.clone();
        let api = Actor::spawn(endpoint, anchor_peers, handler);

        // hydrate our peers list from anchor nodes
        let api2 = api.clone();
        n0_future::task::spawn(async move {
            for anchor in anchors {
                if let Err(e) = api2.inner.rpc(RequestPeerInfos { remote_id: anchor }).await {
                    tracing::error!("requesting peer infos: {:?}", e);
                }
            }
        });

        // re-broadcast our subscriptions every interval
        if peer_info_broadcast_interval > Duration::from_millis(0) {
            let api2 = api.clone();
            n0_future::task::spawn(async move {
                loop {
                    tokio::time::sleep(peer_info_broadcast_interval).await;
                    if let Err(e) = api2.broadcast_peer_info().await {
                        tracing::error!("broadcasting peer info: {:?}", e);
                    }
                }
            });
        }

        // prune stale subscriptione every prune interval
        if peer_prune_interval > Duration::from_millis(0) {
            let api2 = api.clone();
            n0_future::task::spawn(async move {
                loop {
                    tokio::time::sleep(peer_prune_interval).await;
                    let cutoff_timestamp = timestamp() - peer_prune_interval.as_secs();
                    if let Err(e) = api2.inner.rpc(PruneMyPeers { cutoff_timestamp }).await {
                        tracing::error!("pruning stale subscriptions: {:?}", e);
                    }
                }
            });
        }

        api
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

    /// List all peers we know about, and the subscriptions they have
    pub(crate) async fn peers(&self) -> irpc::Result<Vec<PeerInfo>> {
        let mut rx = self.inner.server_streaming(MyPeers {}, 1000).await?;
        let mut peers = Vec::new();
        while let Some(peer) = rx.recv().await? {
            peers.push(peer);
        }
        Ok(peers)
    }

    pub(crate) async fn my_peer_info(&self) -> irpc::Result<PeerInfo> {
        self.inner.rpc(MyPeerInfo {}).await
    }

    async fn broadcast_peer_info(&self) -> irpc::Result<JoinHandle<()>> {
        let peers = self.peers().await?;
        let subs = self.my_peer_info().await?;
        let client = self.inner.clone();
        let handle = n0_future::task::spawn(async move {
            if let Err(e) = broadcast_peer_info_inner(client, peers, subs).await {
                tracing::error!("Peer announcement task failed: {:?}", e);
            }
        });
        Ok(handle)
    }

    pub(crate) async fn leaving(&self) -> irpc::Result<()> {
        self.inner.rpc(BroadcastLeaving {}).await
    }

    pub(crate) async fn subscribe(&self, key: String, self_id: NodeId) -> irpc::Result<()> {
        self.inner
            .rpc(Subscribe {
                key,
                remote_id: self_id,
            })
            .await?;

        // subscription set has changed. announce our updated info to all known peers
        self.broadcast_peer_info().await?;

        Ok(())
    }

    pub(crate) async fn unsubscribe(&self, key: String, self_id: NodeId) -> irpc::Result<()> {
        self.inner
            .rpc(Unsubscribe {
                key,
                remote_id: self_id,
            })
            .await?;

        // subscription set has changed. announce our updated info to all known peers
        self.broadcast_peer_info().await?;

        Ok(())
    }

    /// Send this segment to all subscriptions.
    pub(crate) async fn send_segment(&self, key: String, data: Bytes) -> irpc::Result<()> {
        let msg = SendSegment { key, data };
        self.inner.rpc(msg).await
    }
}

async fn broadcast_peer_info_inner(
    client: Client<Protocol>,
    peers: Vec<PeerInfo>,
    my_subs: PeerInfo,
) -> irpc::Result<()> {
    for peer in peers.iter() {
        client
            .rpc(SendPeerInfo {
                remote_id: peer.node_id,
                info: my_subs.clone(),
            })
            .await?;
    }
    Ok(())
}
