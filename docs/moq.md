# Media over QUIC

> **Work in progress — not safe to deploy.** This branch preserves the MoQ
> cutover for integration work; it is not a completed production migration.
> Release requires cryptographic source/ingredient binding from MUXL, the
> Desktop/OBS low/high/shared-audio producer integration, and an archive-only
> recording sink that respects recording leases and policy. Passing transport
> tests does not establish those missing guarantees.

Streamplace nodes hand live streams to each other over
[moq-lite](https://datatracker.ietf.org/doc/html/draft-lcurley-moq-lite-06)
(draft 06), using `moq.dev/moq v0.7.14` and its pinned Rust FFI v0.4.14.
`pkg/moq` contains Streamplace certificate and MUXL conventions, not a wire
implementation. The listener and replication integration live in
`pkg/replication/websocketrep`. A node
pulls a stream from its origin, preferring the origin's MoQ URL over its
websocket when one is advertised, and pushes its own streams to the nodes
named in `--peers`.

## Wire convention

The payload is MUXL and nothing else.

| moq-lite               | Streamplace                                                                    |
| ---------------------- | ------------------------------------------------------------------------------ |
| broadcast path         | the streamer's DID (`did:plc:...`)                                             |
| track `source`         | the stream's signed source segments                                            |
| track `rendition-<id>` | one independently signed MUXL rendition track, identified by its MUXL track ID |
| group                  | one segment (one GoP)                                                          |
| frame                  | the bare canonical MUXL segment, the bytes `bus.Seg.Muxl` carries              |
| timescale              | 1000000; timestamp is the signed source start time in Unix microseconds        |

The application catalog section `streamplace` lists rendition tracks and the
latest source sequence. Sequences are persistent but origin-local: never reuse a
FETCH cursor against another origin. Each rendition group uses its source group's
sequence at that origin, so its signed ingredient can be checked against the exact
source even when it arrives first.

A subscriber validates source frames with `MediaManager.ValidateReplicatedMP4`,
binding the verified creator to the announced DID before deduplication or live
effects. Rendition signatures are checked, but the current cross-source ingredient
check compares manifest labels, not authenticated manifest digests. This is a
release-blocking integration gap: MUXL must expose cryptographic ingredient/source
identity before the rendition path is safe against a forged provenance chain
reusing a label. Signed source bytes are persisted before publication, outside the
best-effort segment bus; disk failures propagate rather than silently dropping data.

Official subscriptions carry explicit priority and a two-second freshness budget.
`MaxAgeUs` measures presentation-time freshness, not durable cache retention.
The source still carries required audio and metadata; it cannot be cancelled as
though it were video-only. The independently measured low-only fallback policy
and remaining producer/recording integration are documented in [the spike report](moq-spike.md).

`FetchMoqGroup` uses a separate connection and archive-only validation. It stores
late bytes without feeding the live bus, refreshing origins, or acquiring a
recording lease. This is distinct from live ingestion. The Go durable spool backs
uncached FETCH and retains original timestamps across restarts. Retention is the
earlier of the signed delete-after deadline and `SegmentArchiveRetention` from
first receipt; duplicate receipt never extends it. Disk pruning preserves a
durable sequence high-water mark so expired cursors never name new media.

Restart recovery validates retained signed source and exposes an exact-DID dynamic
archive route, without replaying groups, restoring a live catalog, or announcing
the stream to push peers. Finished native group producers, including FETCH cache
fills, are explicitly aborted at expiry so cache hits cannot bypass deletion.
Live readers use arrival order; ordered cache readers can intentionally see
FETCHed history. The spool is bounded transport retention, not a finished VOD or
an alternative recording lease holder.

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
keeps one official publishing session open for as long as it runs. Its local
origin announces one route per streamer DID. The receiver waits for those routes,
then subscribes to `source` and catalogued `rendition-<id>` tracks.
Each incoming connection has a private receive origin, so unvalidated peer
announcements cannot become routes on the public publishing origin.
A pushed stream is a replicated stream on the peer (`Local: false`);
while it flows the peer writes itself into the station's `broadcast_origins`
table for it, so the nodes sharing its statedb pull the stream from the peer
rather than from a pusher they cannot reach.

An `https://` peer is asked for its current MoQ URL on every dial, through
`place.stream.broadcast.getBroadcaster`'s `moqURL`: a node behind an HTTPS
proxy serves MoQ with a self-signed certificate it rotates, so a fixed
`moqt://…?certhash=` entry would go stale. A `moqt://` entry is dialled as
given. A listener accepts any pushed stream its `--allowed-streams` would
accept over RTMP; a stream that fails that check, or a frame that fails
validation, ends the session. Validation refuses a replicated segment whose
start time is more than ten minutes from the node's clock, so a pusher
cannot replay a public streamer's old signed segments to make them appear
live (the same window applies to every replicated segment, `/api/segment`
included).

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
