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

Every byte a broadcast carries is signed MUXL, laid out the way the rest of
the MoQ ecosystem reads media: a hang catalog and one CMAF track per media
track. The MoQ tools (`moq export`, `moq play`) and the `@moq/watch` player
can play a Streamplace broadcast, and a Streamplace node reassembles the
exact signed segments and verifies them as before.

| MoQ                                         | Streamplace                                                                                                 |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| broadcast path                              | the streamer's DID (`did:plc:...`)                                                                          |
| `catalog.json`, `catalog.json.z`, `catalog` | the hang and MSF catalogs, one CMAF rendition per track                                                     |
| track                                       | one MUXL track, named by its MUXL rendition (`video1`, `audio2`, `audio3`; renditions `video100` up)        |
| group                                       | that track's canonical segment for one GoP                                                                  |
| frame                                       | one `moof`+`mdat`, the first also carrying the segment's leading `uuid` boxes (C2PA manifest, MUXL catalog) |
| timestamp                                   | the fragment's first presentation time, on the track's timescale                                            |

A group's frames concatenate back to the track's canonical segment, byte
for byte. Each catalog rendition's CMAF `init` is the track's own init
segment, synthesized from its MUXL catalog (`muxl wrap --init-only`); the
hang codec fields are derived from that init by `moq-mux`. The catalog is
unsigned: a node uses it only to find the tracks, and verifies the bytes.

The source segment's tracks (MUXL track ids below
`media.RenditionTrackID(0)`) share one group sequence per segment. A
subscriber reads each in sequence order, concatenates one segment's
groups in track-id order, and validates the result with
`MediaManager.ValidateReplicatedMP4`: the websocket path's validation,
plus a check that the segment's signed creator is the streamer it asked
for. A source track with no group at a segment's sequence has no part in
that segment. Each rendition track's group is a single-track addendum,
verified with `muxl verify` and folded into the live window on its own.
Anything that is not MUXL ends the session. So does a catalog listing more
than `--moq-max-tracks` tracks (default 32) or a group over
`--moq-max-group-mb` (default 16 MiB): a subscriber buffers a whole group
before it can verify it, so these bound what one peer's stream can make a
node hold. Either is off at 0.

A node's listener advertises one route covering every path and serves a
broadcast when a peer requests it: a request for a stream that is not live
on its segment bus is refused, and a served broadcast ends with the
stream's session, or when a track's codec config changes (its MUXL catalog
box differs), since the catalog entry describes the old one; the next
request starts afresh. The transport shares one served broadcast among all
its subscribers. A track is published the first time a segment carries it,
and a new subscriber starts from the newest segment, like
`place.stream.live.subscribeSegments`'s priming. The source tracks have the
higher subscriber priority, and a subscriber that falls behind skips ahead.

To watch or dump a broadcast with the MoQ CLI (pin a self-signed node's
certificate with the `certhash` from its MoQ URL):

```sh
moq --connect moqt://host:38443 --connect-tls-fingerprint <certhash> \
    --broadcast did:plc:... export fmp4 > stream.mp4
```

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
is ingesting: each is announced, one broadcast per streamer DID, once its
first published segment's tracks are in the catalog, until the stream
session times out, its tracks served from the bus exactly as the listener
serves them. The peer gives every incoming session its own origin, so a
pushed announcement never becomes a route it serves; it pulls each
announced stream's source tracks from that session and validates them
exactly as it would a pull, and ignores
announcements past 32 streams or that are not DIDs. It takes no renditions
from a pusher, whose signer could be anyone: as the stream's origin it mints
its own. A pushed stream is a replicated stream on
the peer (`Local: false`); while it flows the peer writes itself into the
station's `broadcast_origins` table for it, so the nodes sharing its
statedb pull the stream from the peer. The pusher hands its streams' origin
duties to its peers: it writes no `broadcast_origins` row of its own and
does not contend for transcoding (below), so a pusher on the same statedb
as its peer, out of the other nodes' reach, is never chosen over it.

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
stream. The nodes that can do a job contend for it through a lease in the
station's shared statedb (`leases`; `pkg/director/roles.go`). The
holder renews the lease every 2 seconds; a node that dies stops, and the
next contender takes the job over once the 6 second TTL has lapsed. A node
that ends a session cleanly releases its leases, so a peer takes over at
once. A node that wins a job but cannot take it up (a failed lookup before
recording) releases the lease at once and asks again after 10 seconds; one
that will not do it for this stream (the streamer turned recording off)
releases it and stops asking. A node on its own statedb simply holds every
lease.

Any node with a session for a stream contends for its multistream targets.
Recording (S3 configured) and transcoding (a Livepeer gateway) are
contended for only by a node that is an origin for the stream, one that
ingests it without pushing it to `--peers` or that a peer pushes it to: both
want the stream at its live edge. Peers take a stream's renditions from its
origin, so renditions minted anywhere else would reach nobody but that
node's own viewers; and a pull replica, seconds behind, that took the
recording over would record again segments its predecessor had saved. A
recorder completes its current object when the stream stops being
published, so the recording can be finalized at once: an ingest node on its
first unpublished segment, a push receiver, which is pushed no unpublished
segments, on seeing the streamer's livestream record end.

A pusher whose `--peers` names two nodes of one station gives that station
two origins, of which one transcodes. The other gets the renditions only by
pulling the stream from it, so such a station needs `--syndicate`.
