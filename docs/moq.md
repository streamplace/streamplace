# Media over QUIC

Streamplace nodes hand live streams to each other over Media over QUIC,
using the official implementation: `moq.dev/moq`, the Go wrapper over the
Rust `moq-ffi` that Meson builds from source into the node (see
`rust/streamplace-native`). `pkg/moq` holds only what Streamplace adds, the
listener's certificate handling. The listener and the pull side live in the
`websocketrep` replicator (`pkg/replication/websocketrep/moq.go`), which
prefers an origin's MoQ URL over its websocket when one is advertised.

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
Anything that is not MUXL ends the session.

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
