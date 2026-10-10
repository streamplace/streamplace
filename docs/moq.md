# Media over QUIC

Streamplace nodes hand live streams to each other over
[moq-lite](https://datatracker.ietf.org/doc/html/draft-lcurley-moq-lite-06)
(draft 06). The implementation is `pkg/moq`; the node's listener and the
segment-bus publisher are `pkg/api/moq.go`; the pull side is the
`websocketrep` replicator, which prefers an origin's MoQ URL over its
websocket when one is advertised.

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

| Flag         | Env           | Default  | Meaning                                                                                      |
| ------------ | ------------- | -------- | -------------------------------------------------------------------------------------------- |
| `--moq-addr` | `SP_MOQ_ADDR` | `:38443` | UDP listen address; empty disables the listener.                                             |
| `--moq-url`  | `SP_MOQ_URL`  |          | Override for the advertised `moqt://` URL (default: `--server-host` on `--moq-addr`'s port). |

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
