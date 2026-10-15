# Official MoQ Go wrapper integration spike

Branch: `gpt6astra/moq-go-spike`, based on `fable51/moq-push`
(`4014e5a92280e8cb73188f2c0e626598b3489668`). Production MoQ remains unchanged.

## Decision gate

Do not cut over yet. The official transport works when built from source, but the
published native artifacts do not fit Streamplace's release matrix, and even a
locally rebuilt MoQ static library conflicts with `iroh_streamplace`.

The spike is `hack/moq-spike`, an independent pinned Go module. It exercises real
signed MUXL payloads over raw QUIC and WebTransport, separate low/high tracks,
subscription priority, freshness, bandwidth grants, and a fresh-session FETCH
path. It is not a production VOD implementation or a synchronized encoding ladder.

## Version and source evidence

- `go mod download moq.dev/moq@latest` resolved **v0.7.14**.
- Its actual native dependency is **moq.dev/moq-ffi v0.4.14**.
- Native source tag `moq-ffi-v0.4.14` resolves to
  [`7dccdbd37db632bff51f5be86d768ae9e0092dec`](https://github.com/moq-dev/moq/tree/7dccdbd37db632bff51f5be86d768ae9e0092dec).
- Builder: `streamplace-3`, Go 1.26.5, Rust 1.98.1, Ubuntu 22.04/glibc 2.35.

### API claims checked against implementation

| Capability            | Finding                                                                                                                                                                           |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Subscription priority | Higher values are scheduled first; Lite reprioritizes existing streams.                                                                                                           |
| Publisher priority    | Used by bandwidth allocation; the inspected Lite scheduling key does not implement the documented publisher-priority tie-break.                                                   |
| MaxAgeUs              | Presentation-time freshness, newest group exempt; not a wall-clock deadline or retention guarantee.                                                                               |
| FETCH                 | Explicit one-group backfill bypasses live freshness; cache miss needs a dynamic source handler.                                                                                   |
| Reserve/Grant         | Advisory budget, not a policer. Higher publisher-priority tiers first. nil means no estimate/demand; zero is a real zero grant.                                                   |
| Stats                 | Sampled optional fields; counters belong to the current connection, not the lifetime across reconnects.                                                                           |
| Reconnect/origins     | Implemented with backoff and shared routing/fanout; production mesh recovery was not exercised.                                                                                   |
| WebSocket fallback    | Client races HTTP(S) QUIC against QMux/WebSocket. Raw `moqt` does not fallback; the Go server API does not expose a WebSocket listener option. Not exercised here.                |
| Protocol              | Default preference starts with Lite06, then older Lite and IETF revisions. IETF20–22 FETCH is rejected in the inspected source. Go lacks negotiated-revision reporting/selection. |
| TLS                   | File-based identity with live reload and SHA-256 leaf pinning. No Go in-memory certificate callback.                                                                              |

Primary implementations:
[`lite/priority.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-net/src/lite/priority.rs),
[`model/track.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-net/src/model/track.rs),
[`model/bandwidth.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-net/src/model/bandwidth.rs),
[`version.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-net/src/version.rs),
[`ietf/subscriber.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-net/src/ietf/subscriber.rs),
[`tls.rs`](https://github.com/moq-dev/moq/blob/7dccdbd37db632bff51f5be86d768ae9e0092dec/rs/moq-tokio/src/tls.rs).

## Native build results

| Target / composition                   | Observed result                                                                                                              |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Linux amd64, published archive         | Link fails: AWS-LC requires `__isoc23_sscanf` and `__isoc23_strtol`, absent in builder glibc 2.35.                           |
| Linux arm64, published archive         | Same libc symbol failure with configured cross compiler.                                                                     |
| Windows amd64                          | Published MSVC `.lib` fails with Streamplace's MinGW toolchain, including unresolved `__GSHandlerCheck`.                     |
| Intel macOS                            | No archive or cgo link directive; configured osxcross link fails with missing MoQ symbols.                                   |
| Apple Silicon macOS                    | Links with osxcross, but numerous objects warn they target macOS 26.5; deployment compatibility and execution untested.      |
| Linux amd64, rebuilt from exact source | Builds and runs using upstream `rs/moq-ffi/build.sh` with default features.                                                  |
| MoQ + Iroh static libraries            | Duplicate Rust std/hashbrown symbols, both with published and locally rebuilt MoQ. No duplicate-definition suppression used. |

The Go FFI module contains no Android/iOS archives. This is a future Go-embedding
constraint, not a demonstrated break of today's Expo app: no `js/app` consumer of
Meson's `streamplace.a` was found. Rust's mobile build scripts alone do not prove
Go-wrapper mobile linkage.

The published archives total 228,909,354 bytes. An unstripped Apple Silicon minimal
Go executable was 1,749,218 bytes versus 28,368,034 bytes for the wrapper listener.
The source-built Linux transport probe was 33,549,808 bytes. These are probe sizes,
not measured incremental size in a production Streamplace binary.

Disabling native audio/video features is not a drop-in size fix: published Go
bindings unconditionally reference their checksum/export symbols. Regenerating a
transport-only binding also requires adapting the ergonomic wrapper.

## TLS runtime proof

Source-built raw QUIC and WebTransport connected with the correct pin; a wrong pin
failed explicitly with a cryptographic fingerprint mismatch. A rotating file test
verified that an intermediate mismatched cert/key retains the old identity, then a
valid replacement updates the fingerprint and accepts a newly pinned connection.

`Server.CertFingerprints()` returned **Busy** while `Accept()` was blocked. The
native certificate observer supports live updates, but the FFI shares one Task for
accept and inspection. Rotation worked with serialized calls; concurrent discovery
needs an upstream API correction or a deliberately designed integration boundary.

## Congestion findings

All runs use real signed fixture bytes and advancing transport timestamps. The
20-second run publishes 20 low groups and 25 high groups. A disposable user/network
namespace applies `tc netem rate 2mbit delay 20ms` to loopback, including backfill.

Priority plus MaxAge alone was insufficient:

| Initial policy       | Low received |  Low p95 | High received live | High fetched afterwards |
| -------------------- | -----------: | -------: | -----------------: | ----------------------: |
| Raw QUIC, unshaped   |        20/20 | 2.543 ms |              25/25 |                   25/25 |
| Raw QUIC, shaped     |        18/20 |  3536 ms |               4/25 |                    1/25 |
| WebTransport, shaped |        20/20 |  1258 ms |               4/25 |                    7/25 |

The high live subscription drained long after publication ended; the default
origin's approximate 30-second idle cache expiry discarded most groups before
backfill began. Large publisher MaxAge did not preserve them. Some high groups
completed roughly 50 seconds late, so they cannot be treated as live video.

The spike now explicitly cancels high live demand when the receive-rate estimate
cannot sustain high, rejects late high completions from live results, and serves
uncached FETCH requests from a separate run-lifetime archive. This archive retains
immutable signed bytes and timestamps; production must connect the same mechanism
to durable storage rather than rely on MoQ's cache.

With explicit fallback, both shaped transports passed:

| Final policy         | Low received |    Low p95 | High received live | High fetched afterwards |
| -------------------- | -----------: | ---------: | -----------------: | ----------------------: |
| Raw QUIC, shaped     |        20/20 | 651.600 ms |               0/25 |                   25/25 |
| WebTransport, shaped |        20/20 | 655.972 ms |               0/25 |                   25/25 |

The raw QUIC fallback observed a 2,024,381 bit/s receive estimate against the
12 Mbit/s high reservation. Every fetched payload, size and timestamp matched;
no fetched group entered live results. These immediate FETCH runs hit the cache,
so they do not alone prove the dynamic archive handler.

A separate run with a 2 MiB origin cache forced **8 dynamic archive reads**;
all 10 high groups were fetched unchanged. The final spike keeps this small
cache so archive recovery is exercised rather than inferred. A 40-second
shaped run also delivered all 40 low groups and fetched all 50 high groups.

## Verification and next gate

`make provision` passed all 26 existing Web E2E tests, including current MoQ pull
and push playback. `go test -count=1 ./pkg/moq/...` passed. These establish the
unchanged base, not the new transport cutover. Standalone baseline and race-enabled
smokes exercised both live tracks and complete FETCH over a new session.
`make check` passed after moving investigation sources out of the checkout;
the initial attempt failed because its recursive gofmt scan included those
unmodified upstream files. The independent spike module also passed `go vet` and
compiled under `go test -count=1` (no permanent test files); actual verification
comes from the network runs, including a race-enabled smoke with no Go race report.

Before production cutover:

1. Choose a supported source-built native composition that avoids two independent
   Rust static runtimes, and prove every release target. Do not add linker shims.
2. Eli selected **MoQ Lite** for the cutover; the current IETF FETCH limitation
   therefore does not block the selected protocol, but must remain documented.
3. Eli selected **Desktop / OBS multitrack** encoding. Agree its signed
   rendition/catalog contract with MTRACK before integration.
4. Define durable high-rendition retention and a backfill-only ingestion path;
   never feed delayed archive groups through the live replication bus.
5. Preserve Go validation, creator-to-announcement binding, allowed-streams,
   discovery, and job leases in the eventual cutover.

Could this be smaller? The spike reuses the upstream transport, MUXL signer, and
existing fixtures, and keeps all experiments out of production. The eventual
cutover should delete the bespoke wire stack and consolidate native Rust linkage,
not introduce a second permanent transport implementation.
