# Media over QUIC

Streamplace nodes hand live streams to each other over Media over QUIC,
using the official implementation: `moq.dev/moq`, the Go wrapper over the
Rust `moq-ffi` that Meson builds from source into the node (see
`rust/streamplace-native`). `pkg/moq` holds only what Streamplace adds, the
listener's certificate handling. The listener and both directions of
transfer live in the `websocketrep` replicator
(`pkg/replication/websocketrep/moq.go`). A node pulls a stream from its
origin, preferring the origin's MoQ URL over its websocket when one is
advertised, and pushes its own streams to the nodes named in `--peers`.

## Wire convention

The payload is MUXL and nothing else.

| MoQ                | Streamplace                                                             |
| ------------------ | ----------------------------------------------------------------------- |
| broadcast path     | the streamer's DID (`did:plc:...`)                                      |
| track `source`     | the stream's signed source segments                                     |
| track `renditions` | the rendition addenda the ingest node mints (`media.RenditionsChannel`) |
| group              | one segment (one GoP)                                                   |
| frame              | the bare canonical MUXL segment, the bytes `bus.Seg.Muxl` carries       |
| timestamp          | the publish time in Unix microseconds                                   |

A subscriber reads each track in sequence order and validates every source
frame with `MediaManager.ValidateReplicatedMP4`: the websocket path's
validation, plus a check that the segment's signed creator is the streamer
it asked for. It verifies every rendition addendum with `muxl verify`. A
frame that is not MUXL, or a group holding more than one, ends the session;
the publisher ends a track rather than send a segment that does not lead
with a `uuid` box at all.

A node's listener advertises one route covering every path and serves a
broadcast when a peer requests it: a request for a stream that is not live
on its segment bus is refused, and a served broadcast ends with the
stream's session. The transport shares one served broadcast among all its
subscribers. Each requested track is fed from the bus for as long as the
broadcast lasts, and a new subscriber starts from the newest segment, like
`place.stream.live.subscribeSegments`'s priming. The source track has the
higher priority, and a subscriber more than ten seconds behind skips ahead.

## Bindings

One UDP port serves raw QUIC (`moqt://host:port`) and WebTransport, which
is the binding a browser uses. The transport negotiates the protocol
version by ALPN.

## Configuration

| Flag         | Env           | Default  | Meaning                                                                                                        |
| ------------ | ------------- | -------- | -------------------------------------------------------------------------------------------------------------- |
| `--moq-addr` | `SP_MOQ_ADDR` | `:38443` | UDP listen address; empty disables the listener.                                                               |
| `--moq-url`  | `SP_MOQ_URL`  |          | Override for the advertised `moqt://` URL (default: `--server-host` on `--moq-addr`'s port).                   |
| `--peers`    | `SP_PEERS`    |          | Nodes to push this node's live streams to: each an `https://` node origin or a `moqt://` URL, comma-separated. |

The listener terminates TLS itself, even on a node behind an HTTPS proxy,
reading its certificate from files (the data directory's `moq-tls`) that
Go keeps current:

- with `--secure`, it serves the node's certificate (ACME, rewritten as it
  renews, or `--tls-cert`/`--tls-key` as given);
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
keeps one session open for as long as it runs, publishing the streams it
is ingesting: each is announced, one broadcast per streamer DID, from its
first segment until the stream session times out, its tracks served from
the bus exactly as the listener serves them. The peer gives every
incoming session its own origin, so a pushed announcement never becomes
a route it serves; it pulls each announced stream from that session and
validates it exactly as it would a pull, and ignores announcements past 32
streams or that are not DIDs. A pushed stream is a replicated stream on
the peer (`Local: false`); while it flows the peer writes itself into the
station's `broadcast_origins` table for it, so the nodes sharing its
statedb pull the stream from the peer rather than from a pusher they
cannot reach.

An `https://` peer is asked for its current MoQ URL on every dial, through
`place.stream.broadcast.getBroadcaster`'s `moqURL`: a node behind an HTTPS
proxy serves MoQ with a self-signed certificate it rotates, so a fixed
`moqt://…?certhash=` entry would go stale. A `moqt://` entry is dialled as
given. A listener accepts any pushed stream its `--allowed-streams` would
accept over RTMP; a frame that fails validation ends the session.
Validation refuses a replicated segment whose start time is more than ten
minutes from the node's clock, so a pusher cannot replay a public
streamer's old signed segments to make them appear live (the same window
applies to every replicated segment, `/api/segment` included).

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
