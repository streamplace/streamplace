use std::{collections::BTreeSet, ops::Deref, sync::Arc, time::Duration};

use async_trait::async_trait;
use n0_future::{BufferedStreamExt, StreamExt, stream};
use testresult::TestResult;

use super::*;

struct TestNode {
    node: Arc<Node>,
    public: Arc<crate::public_key::PublicKey>,
    ticket: String,
    #[allow(dead_code)]
    private: Vec<u8>,
}

impl Deref for TestNode {
    type Target = Node;

    fn deref(&self) -> &Self::Target {
        &self.node
    }
}

impl TestNode {
    /// Helper to create a test node with given config and handler mode.
    async fn new(handler: HandlerMode) -> TestResult<TestNode> {
        let config = Config {
            key: vec![0_u8; 32],   // will be replaced
            topic: vec![0_u8; 32], // all nodes use the same topic
            max_send_duration: Duration::from_secs(10),
            disable_relay: false,
        };
        Self::new_with_config(handler, config).await
    }

    async fn new_with_config(handler: HandlerMode, mut config: Config) -> TestResult<TestNode> {
        let key = iroh::SecretKey::generate();
        let key = key.to_bytes().to_vec();
        config.key = key.clone();
        let node = Node::new_in_runtime(config, handler).await?;
        let public = node.node_id().await?;
        let ticket = node.ticket().await?;
        Ok(TestNode {
            node,
            private: key,
            public,
            ticket,
        })
    }
}

/// Helper to create multiple test nodes with given handler mode.
async fn test_nodes(
    n: usize,
    handler: impl Fn(usize) -> HandlerMode + Send + Sync,
    disable_relay: bool,
) -> TestResult<Vec<TestNode>> {
    const PAR: usize = 32;
    let modes = (0..n).map(handler).collect::<Vec<_>>();

    let config = Config {
        key: vec![0_u8; 32], // will be replaced
        topic: vec![0_u8; 32],
        max_send_duration: Duration::from_secs(10),
        disable_relay,
    };
    // Preserve role/index order while initializing endpoints concurrently.
    let nodes = stream::iter(modes)
        .map(|mode| TestNode::new_with_config(mode, config.clone()))
        .buffered_ordered(PAR)
        .collect::<Vec<_>>()
        .await;
    let nodes = nodes.into_iter().collect::<TestResult<Vec<_>>>()?;
    // join everyone to everyone
    let tickets = nodes.iter().map(|n| n.ticket.clone()).collect::<Vec<_>>();
    let res = stream::iter(&nodes)
        .map(|n| n.join_peers(tickets.clone()))
        .buffered_unordered(PAR)
        .collect::<Vec<_>>()
        .await;
    res.into_iter().collect::<Result<Vec<_>, _>>()?;
    Ok(nodes)
}

#[tokio::test]
async fn one_node() -> TestResult<()> {
    tracing_subscriber::fmt::try_init().ok();
    let node = TestNode::new(HandlerMode::Sender).await?.node;
    let write = node.node_scope();
    let db = node.db();
    println!("Ticket: {}", node.ticket().await?);
    write
        .put(Some(b"stream1".to_vec()), b"s".to_vec(), b"y".to_vec())
        .await?;
    write
        .put(Some(b"stream2".to_vec()), b"s".to_vec(), b"y".to_vec())
        .await?;
    let res = db.subscribe_with_opts(SubscribeOpts {
        filter: Filter::new(),
        mode: SubscribeMode::Both,
    });
    while let Some(item) = res.next_raw().await? {
        if let SubscribeItem::CurrentDone = item {
            break;
        }
        println!("Got item: {item:?}");
    }
    let res = db
        .iter_with_opts(
            Filter::new()
                .stream(b"stream1".to_vec())
                .scope(node.node_id().await?),
        )
        .await?;
    println!("Iter result: {res:?}");
    node.shutdown().await?;
    Ok(())
}

struct TestHandler<T> {
    info: T,
    sender: tokio::sync::mpsc::Sender<(T, String, Vec<u8>)>,
}

impl<T> TestHandler<T> {
    fn new(info: T, sender: tokio::sync::mpsc::Sender<(T, String, Vec<u8>)>) -> Self {
        Self { info, sender }
    }
}

#[async_trait]
impl<T: Clone + Send + Sync + 'static> DataHandler for TestHandler<T> {
    async fn handle_data(&self, _from: Arc<public_key::PublicKey>, topic: String, data: Vec<u8>) {
        self.sender
            .send((self.info.clone(), topic, data))
            .await
            .ok();
    }
}

/// Deadline for one segment to travel through the test topology. A lost delivery must
/// fail the test instead of blocking on the channel forever.
const RECV_TIMEOUT: Duration = Duration::from_secs(10);

/// Deadline for the metadata db of one node to converge through gossip. Records that
/// predate a gossip neighbor only arrive through anti-entropy, which republishes
/// everything within `iroh_smol_kv::Config::default().fast_anti_entropy_interval`.
const CONVERGE_TIMEOUT: Duration = Duration::from_secs(60);

async fn recv_data<T>(
    rx: &mut tokio::sync::mpsc::Receiver<(T, String, Vec<u8>)>,
) -> TestResult<(T, String, Vec<u8>)> {
    Ok(tokio::time::timeout(RECV_TIMEOUT, rx.recv())
        .await?
        .expect("should get data"))
}

#[tokio::test]
async fn two_nodes_send_receive() -> TestResult<()> {
    tracing_subscriber::fmt::try_init().ok();
    let (tx, mut rx) = tokio::sync::mpsc::channel(32);
    let handler = Arc::new(TestHandler::new((), tx));
    let config = Config {
        key: vec![0; 32],
        topic: vec![0; 32],
        max_send_duration: Duration::from_secs(10),
        disable_relay: true,
    };
    let sender = TestNode::new_with_config(HandlerMode::Sender, config.clone()).await?;
    let receiver = TestNode::new_with_config(HandlerMode::Receiver(handler), config).await?;
    assert!(sender.ticket.starts_with("endpoint"));
    assert_eq!(
        node_addr::node_id_from_ticket(sender.ticket.clone())?,
        sender.public
    );
    // join the sender to the receiver. This will also configure the receiver endpoint to be able to dial the sender.
    receiver.join_peers(vec![sender.ticket.clone()]).await?;
    let stream = "teststream".to_string();
    receiver
        .subscribe(stream.clone(), sender.public.clone())
        .await?;
    sender.send_segment(stream, b"segment1".to_vec()).await?;
    let (_, stream, data) = recv_data(&mut rx).await?;
    assert_eq!(stream, "teststream");
    assert_eq!(data, b"segment1".to_vec());
    receiver.shutdown().await?;
    sender.shutdown().await?;
    Ok(())
}

#[tokio::test]
async fn subscribe_without_peer_address_fails() -> TestResult<()> {
    let config = Config {
        key: vec![0; 32],
        topic: vec![0; 32],
        max_send_duration: Duration::from_secs(1),
        disable_relay: true,
    };
    let sender = TestNode::new_with_config(HandlerMode::Sender, config.clone()).await?;
    let receiver = TestNode::new_with_config(HandlerMode::Sender, config).await?;
    // No ticket was exchanged, so the remote cannot register this subscription.
    let result = tokio::time::timeout(
        Duration::from_secs(5),
        receiver.subscribe("missing".to_string(), sender.public.clone()),
    )
    .await?;
    assert!(matches!(result, Err(PutError::Irpc { .. })));
    receiver.shutdown().await?;
    sender.shutdown().await?;
    Ok(())
}

#[tokio::test]
async fn three_nodes_send_forward_receive() -> TestResult<()> {
    tracing_subscriber::fmt::try_init().ok();
    let (tx, mut rx) = tokio::sync::mpsc::channel(32);
    let handler = Arc::new(TestHandler::new((), tx));
    let sender = TestNode::new(HandlerMode::Sender).await?;
    let forwarder = TestNode::new(HandlerMode::Forwarder).await?;
    let receiver = TestNode::new(HandlerMode::Receiver(handler)).await?;
    // join everyone to everyone, so the receiver can reach the sender via the forwarder.
    let tickets = vec![
        sender.ticket.clone(),
        forwarder.ticket.clone(),
        receiver.ticket.clone(),
    ];
    receiver.join_peers(tickets.clone()).await?;
    forwarder.join_peers(tickets.clone()).await?;
    sender.join_peers(tickets).await?;
    let stream = "teststream".to_string();
    receiver
        .subscribe(stream.clone(), forwarder.public.clone())
        .await?;
    forwarder
        .subscribe(stream.clone(), sender.public.clone())
        .await?;
    sender.send_segment(stream, b"segment1".to_vec()).await?;
    let (_, stream, data) = recv_data(&mut rx).await?;
    assert_eq!(stream, "teststream");
    assert_eq!(data, b"segment1".to_vec());
    receiver.shutdown().await?;
    forwarder.shutdown().await?;
    sender.shutdown().await?;
    Ok(())
}

#[tokio::test]
async fn meta_three_nodes_send_forward_receive() -> TestResult<()> {
    tracing_subscriber::fmt::try_init().ok();
    let (tx, mut rx) = tokio::sync::mpsc::channel(32);
    let handler = Arc::new(TestHandler::new((), tx));
    let sender = TestNode::new(HandlerMode::Sender).await?;
    let forwarder = TestNode::new(HandlerMode::Forwarder).await?;
    let receiver = TestNode::new(HandlerMode::Receiver(handler)).await?;
    // join everyone to everyone, so the receiver can reach the sender via the forwarder.
    let tickets = vec![
        sender.ticket.clone(),
        forwarder.ticket.clone(),
        receiver.ticket.clone(),
    ];
    receiver.join_peers(tickets.clone()).await?;
    forwarder.join_peers(tickets.clone()).await?;
    sender.join_peers(tickets).await?;
    let stream = "teststream".to_string();
    receiver
        .subscribe(stream.clone(), forwarder.public.clone())
        .await?;
    forwarder
        .subscribe(stream.clone(), sender.public.clone())
        .await?;
    sender
        .send_segment(stream.clone(), b"segment1".to_vec())
        .await?;
    let (_, got_stream, data) = recv_data(&mut rx).await?;
    assert_eq!(got_stream, "teststream");
    assert_eq!(data, b"segment1".to_vec());
    // Every node publishes its mode globally at startup, and a node that gains a
    // subscriber publishes the subscriber count under the stream. The receiver only
    // learns the other nodes' records through gossip, so its db must converge to
    // exactly this set. A `Both` subscription never ends on its own, so read it only
    // until the last expected record has arrived.
    let mode = |node: &TestNode, mode: &str| {
        (
            *node.public,
            None,
            b"mode".to_vec(),
            mode.as_bytes().to_vec(),
        )
    };
    let subscribers = |node: &TestNode| {
        (
            *node.public,
            Some(stream.clone().into_bytes()),
            b"subscribers".to_vec(),
            b"1".to_vec(),
        )
    };
    let mut expected = BTreeSet::from([
        mode(&sender, "sender"),
        mode(&forwarder, "forwarder"),
        mode(&receiver, "receiver"),
        subscribers(&sender),
        subscribers(&forwarder),
    ]);
    let updates = receiver.db().subscribe(Filter::new());
    tokio::time::timeout(CONVERGE_TIMEOUT, async {
        while !expected.is_empty() {
            let item = updates
                .next_raw()
                .await?
                .expect("db subscription closed before the metadata converged");
            println!("{}", subscribe_item_debug(&item));
            if let SubscribeItem::Entry {
                scope,
                stream,
                key,
                value,
                ..
            } = item
            {
                expected.remove(&(*scope, stream, key, value));
            }
        }
        Ok::<_, SubscribeNextError>(())
    })
    .await??;
    receiver.shutdown().await?;
    forwarder.shutdown().await?;
    sender.shutdown().await?;
    Ok(())
}

async fn broadcast(
    nsenders: usize,
    nforwarders: usize,
    nreceivers: usize,
    nmsgs: usize,
) -> TestResult<()> {
    let (tx, mut rx) = tokio::sync::mpsc::channel(32);
    let ntotal = nsenders + nforwarders + nreceivers;
    let senders = 0..nsenders;
    let forwarders = nsenders..(nsenders + nforwarders);
    let receivers = (nsenders + nforwarders)..ntotal;
    let make_handler = |i: usize| {
        if senders.contains(&i) {
            HandlerMode::Sender
        } else if forwarders.contains(&i) {
            HandlerMode::Forwarder
        } else {
            HandlerMode::Receiver(Arc::new(TestHandler::new(i, tx.clone())))
        }
    };
    let nodes = test_nodes(ntotal, make_handler, true).await?;
    let senders = &nodes[senders];
    let forwarders = &nodes[forwarders];
    let receivers = &nodes[receivers];
    let stream = "teststream".to_string();
    // subscribe all forwarders to a sender, round robin
    for (i, forwarder) in forwarders.iter().enumerate() {
        let sender = &senders[i % senders.len()];
        forwarder
            .subscribe(stream.clone(), sender.public.clone())
            .await?;
    }
    // subscribe all receivers to a forwarder, round robin
    for (i, receiver) in receivers.iter().enumerate() {
        let forwarder = &forwarders[i % forwarders.len()];
        receiver
            .subscribe(stream.clone(), forwarder.public.clone())
            .await?;
    }
    for _ in 0..nmsgs {
        for sender in senders {
            sender
                .send_segment(stream.clone(), b"segment1".to_vec())
                .await?;
        }
        let mut pending = ((nsenders + nforwarders)..ntotal).collect::<BTreeSet<_>>();
        for _ in 0..receivers.len() {
            let (i, got_stream, data) = recv_data(&mut rx).await?;
            assert_eq!(got_stream, stream);
            assert_eq!(data, b"segment1");
            assert!(pending.remove(&i), "duplicate delivery to receiver {i}");
        }
    }
    for node in nodes {
        node.shutdown().await?;
    }
    Ok(())
}

#[tokio::test]
async fn broadcast_1_2_4() -> TestResult<()> {
    tracing_subscriber::fmt().try_init().ok();
    broadcast(1, 2, 4, 1).await?;
    Ok(())
}

#[tokio::test]
async fn broadcast_1_3_9() -> TestResult<()> {
    tracing_subscriber::fmt().try_init().ok();
    broadcast(1, 3, 9, 1).await?;
    Ok(())
}

#[tokio::test]
async fn broadcast_1_4_16() -> TestResult<()> {
    tracing_subscriber::fmt().try_init().ok();
    broadcast(1, 4, 16, 100).await?;
    Ok(())
}
