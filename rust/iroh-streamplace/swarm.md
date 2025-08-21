# Swarm

Streamplace nodes can participate in a _swarm_, the goal of which is to maintain up-to-date information about other peers in the network, and what they're doing. Each node maintains a list of peers it knows about. peers exchange `PeerInfo`, a data structure that informs other nodes about peer state they may be interested in. It looks like this:

```js
// PeerInfo:
{
  "timestamp": 1234567,  // unix timestamp of when this record was created
                         // nodes set this locally when they receive PeerInfo
                         // records.
  "node_id": "",         // the iroh nodeID of this peer.
  "subscriptions": [""]  // an array of any streamplace feeds
                         // this node is currently replicating.
}
```

These messages are sent between nodes using a custom `iroh-streamplace` protocol, built on irpc. Messages are serialized in [postcard] format, and are sent at specific moments in the process lifecycle:

* at startup, peers will send `PeerInfo` to any pre-configured _anchor peers_
* the response to `PeerInfo` is a an array of NodeIDs `[NodeID]`, which h
* when a node subscribes to a feed, it broadcasts it's updated `PeerInfo` to all known peers
* when a node unsubscribes from a feed, it broadcasts it's updated `PeerInfo` to all known peers
* every `DEFAULT_PEER_INFO_REPUBLISH_INTERVAL`, a node broadcasts it's current `PeerInfo` to all known peers

every `DEFAULT_PEER_PRUNE_INTERVAL`, nodes will examine their local list of peers, and prune any who's latest timestamp is older than the current time, minus the prune interval, this is to purge peers that die off without notice

### Anchor Peers
Anchor peers are _always_ transmitted to. They're expcted to be high-availability nodes. Any broadcast message will always try to

### FFI API
The FFI API to goland is a single method on the `Receiver`: `peers`. It returns an array of `PeerInfo`, representing the nodes current view of the swarm.
