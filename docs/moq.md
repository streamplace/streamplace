# Media over QUIC

Streamplace nodes hand live streams to each other over Media over QUIC,
using the official implementation: `moq.dev/moq`, the Go wrapper over the
Rust `moq-ffi` that Meson builds from source into the node (see
`rust/streamplace-native`). `pkg/moq` holds only what Streamplace adds, the
listener's certificate handling. The listener and the pull side live in the
`websocketrep` replicator (`pkg/replication/websocketrep/moq.go`), which
prefers an origin's MoQ URL over its websocket when one is advertised.

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

| Flag         | Env           | Default  | Meaning                                                                                      |
| ------------ | ------------- | -------- | -------------------------------------------------------------------------------------------- |
| `--moq-addr` | `SP_MOQ_ADDR` | `:38443` | UDP listen address; empty disables the listener.                                             |
| `--moq-url`  | `SP_MOQ_URL`  |          | Override for the advertised `moqt://` URL (default: `--server-host` on `--moq-addr`'s port). |

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
