# Media over QUIC

Streamplace nodes hand live streams to each other over
[moq-lite](https://datatracker.ietf.org/doc/html/draft-lcurley-moq-lite-06)
(draft 06). The implementation is `pkg/moq`; the node's listener, the
segment-bus publisher and both directions of transfer live in the
`websocketrep` replicator (`pkg/replication/websocketrep/moq.go`). A node
pulls a stream from its origin, preferring the origin's MoQ URL over its
websocket when one is advertised, and pushes its own streams to the nodes
named in `--peers`.

## Wire convention

The payload is MUXL and nothing else.

| moq-lite           | Streamplace                                                             |
| ------------------ | ----------------------------------------------------------------------- |
| broadcast path     | the streamer's DID (`did:plc:...`)                                      |
| track `source`     | the stream's signed source segments                                     |
| track `renditions` | the rendition addenda the ingest node mints (`media.RenditionsChannel`) |
| group              | one segment (one GoP)                                                   |
| frame              | the bare canonical MUXL segment, the bytes `bus.Seg.Muxl` carries       |
| timescale          | 1000; a frame's timestamp is the publish time in Unix milliseconds      |

A subscriber validates every source frame with `MediaManager.ValidateMP4`,
exactly as the websocket path does, and verifies every rendition addendum
with `muxl verify`. A frame that is not MUXL fails validation and ends the
session; the publisher side refuses to send a segment that does not lead
with a `uuid` box at all.

Subscriptions start at the live edge: the publisher primes a new subscriber
with the last few segments, like `place.stream.live.subscribeSegments`.
Every group is served whole, in order, on its own QUIC stream; FETCH and
PROBE are refused, and nothing is sent as a datagram.

## Bindings

One UDP port serves both bindings, selected by ALPN:

- raw QUIC with ALPN `moq-lite-06` (`moqt://host:port`), which the SETUP
  Path parameter carries the URL's path and query over;
- WebTransport over HTTP/3 (`https://host:port/<any path>`), with
  `moq-lite-06` negotiated as the WebTransport subprotocol. This is the
  binding a browser uses.

## Configuration

| Flag         | Env           | Default  | Meaning                                                                                                        |
| ------------ | ------------- | -------- | -------------------------------------------------------------------------------------------------------------- |
| `--moq-addr` | `SP_MOQ_ADDR` | `:38443` | UDP listen address; empty disables the listener.                                                               |
| `--moq-url`  | `SP_MOQ_URL`  |          | Override for the advertised `moqt://` URL (default: `--server-host` on `--moq-addr`'s port).                   |
| `--peers`    | `SP_PEERS`    |          | Nodes to push this node's live streams to: each an `https://` node origin or a `moqt://` URL, comma-separated. |

The listener terminates TLS itself, even on a node behind an HTTPS proxy:

- with `--secure`, it serves the node's certificate (ACME or
  `--tls-cert`/`--tls-key`);
- otherwise it mints a self-signed ECDSA certificate, valid for two weeks
  and rotated weekly (the limits a browser puts on a pinned certificate),
  and advertises its SHA-256 as the `certhash` query parameter of the MoQ
  URL. A subscriber pins exactly that certificate.

## Discovery

The advertised URL travels two ways, like the websocket URL:

- the `moqURL` field of the streamer's `place.stream.broadcast.origin`
  record on the firehose;
- the `moq_url` column of the station's shared `broadcast_origins` table,
  written by the ingest node on every local segment, so nodes sharing a
  statedb find each other without a relay.

## Push

A node that cannot be reached, one beside a streamer's encoder behind NAT,
pushes instead of waiting to be pulled from. For each `--peers` entry it
keeps one session open, dialled with the publisher Role in SETUP
(draft §7.3.3), for as long as it runs. The peer answers its
`ANNOUNCE_REQUEST` with the streams the pusher is ingesting, one route per
streamer DID, as they start and stop; the peer subscribes to each one's
`source` and `renditions` tracks and validates them exactly as it would a
pull. A pushed stream is a replicated stream on the peer (`Local: false`);
while it flows the peer writes itself into the station's `broadcast_origins`
table for it, so the nodes sharing its statedb pull the stream from the peer
rather than from a pusher they cannot reach.

An `https://` peer is asked for its current MoQ URL on every dial, through
`place.stream.broadcast.getBroadcaster`'s `moqURL`: a node behind an HTTPS
proxy serves MoQ with a self-signed certificate it rotates, so a fixed
`moqt://…?certhash=` entry would go stale. A `moqt://` entry is dialled as
given. A listener accepts any pushed stream its `--allowed-streams` would
accept over RTMP; a stream that fails that check, or a frame that fails
validation, ends the session.

## Who records, multistreams and transcodes

Recording a stream to S3, driving its multistream targets and transcoding
its renditions must each happen on exactly one node, however many carry the
stream. Every node with a session for a stream, whether it ingested the
segments or received them from a peer, contends for each job it is equipped
to do (S3 configured for recording, always for multistream) through a lease
in the station's shared statedb (`leases`; `pkg/director/roles.go`). The
holder renews the lease every 2 seconds; a node that dies stops, and the
next contender takes the job over once the 6 second TTL has lapsed. A node
that ends a session cleanly releases its leases, so a peer takes over at
once. A node on its own statedb simply holds every lease.

Transcoding is contended for only by a node that is an origin for the
stream, one that ingests it or that a peer pushes it to, and that has a
Livepeer gateway: peers take a stream's renditions from its origin, so
renditions minted on a pull-only replica would reach nobody but its own
viewers.
