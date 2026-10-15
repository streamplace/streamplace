# MoQ transport spike

Isolated evaluation of `moq.dev/moq v0.7.14` / `moq.dev/moq-ffi v0.4.14`.
No production transport or root dependency changes. Run in the Streamplace builder.

## Build

The published Linux archives require newer glibc than the Ubuntu 22.04 builder.
Build the exact upstream source rather than patching libc symbols. Keep upstream
sources outside the Streamplace checkout: `make check` scans ignored Go files too.

```sh
REPO="$PWD"
SPIKE="$(mktemp -d /tmp/streamplace-moq-spike.XXXXXX)"
git clone --depth 1 --branch moq-ffi-v0.4.14 https://github.com/moq-dev/moq.git "$SPIKE/upstream"
# Tag resolves to 7dccdbd37db632bff51f5be86d768ae9e0092dec.
(cd "$SPIKE/upstream" && bash rs/moq-ffi/build.sh --target x86_64-unknown-linux-gnu --output "$SPIKE/native")
mkdir "$SPIKE/static"
cp "$SPIKE/native/moq-ffi-0.4.14-x86_64-unknown-linux-gnu/lib/libmoq_ffi.a" "$SPIKE/static/"
(cd hack/moq-spike && CGO_LDFLAGS="-L$SPIKE/static" go build -o "$SPIKE/moq-spike" .)
```

The static-only directory selects the rebuilt archive before the module's archive
without modifying the Go module cache or introducing a filesystem replacement.
Default native features are intentional: the published Go bindings reference codec
exports even for a transport-only consumer. Disabling audio/video requires changes
to the bindings and wrapper, not just a different archive.

## Run

Run from the repository root. One process hosts two peers using real network
sessions; the backfill uses a fresh connection. The generated certificate is pinned,
not accepted with TLS verification disabled.

```sh
"$SPIKE/moq-spike" -duration 20s -scheme moqt > baseline.json
export SPIKE
unshare -Urn sh -c 'ip link set lo up \
  && tc qdisc add dev lo root netem rate 2mbit delay 20ms \
  && "$SPIKE/moq-spike" -duration 20s -scheme moqt' > shaped.json
```

Repeat with `-scheme https` for WebTransport. The disposable namespace shapes both
directions, including FETCH, without changing the builder or host network.
`iproute2` and permission to create a user/network namespace are required.

## Payload and policy

- Existing MP4 fixtures are canonicalized and S2PA-signed through MUXL WASM with an
  ephemeral secp256k1 identity. One signed GoP per group, one bare MUXL frame.
- Low fixture: `h264-opus-frag.mp4`; high: `sample-segment.mp4`. The signed low
  payload is about 0.93 Mbit/s; high about 10.25 Mbit/s. These are independent
  fixtures, not a synchronized rendition ladder of one video.
- Signing happens once and payloads loop. Per-group transport timestamps advance;
  this measures transport, not production ingest validation or VOD assembly.
- Publisher and subscription priorities favor low (200) over high (100).
  `Reserve` ceilings are 1 and 12 Mbit/s; grants and session statistics are logged.
  Reservations are advisory, not traffic policers.
- Once the receive-rate estimate cannot sustain the high reservation, the live
  subscriber cancels high and keeps low. This one-way fallback is deliberate for
  the spike, not a production adaptive-bitrate controller.
- Late completed high groups are excluded from live results. Backfill never adds
  frames to live results.
- A run-lifetime archive retains the exact high payload/timestamp per sequence.
  `TrackProducer.Dynamic` serves cache misses from that archive. It reuses immutable
  fixture bytes; it is not durable storage. Production requires an archival store.
- `MaxAgeUs` bounds presentation-time freshness, not cache lifetime. The default
  origin's approximate 30-second idle expiry makes cache-only complete VOD recovery
  unsafe, regardless of publisher MaxAge. Explicit FETCH bypasses live freshness.

## Acceptance and output

stderr contains JSON events; stdout contains the complete publisher/subscriber
report. Exit 0 means all checks passed, 1 an acceptance check failed, 2 a run error.

Checks: every low group arrives unchanged, low p95 completion latency is at most
1.5 seconds, and every high group is fetched unchanged over the second session.
SHA-256, byte length and timestamp must match the publisher manifest. The negative
past-final FETCH is reported separately; upstream's error text and sentinel
classification may disagree.

`-h` lists options. Default publication lasts 20 seconds; publisher linger and
backfill are bounded at 3 minutes. Latency assumes the peers share a clock. No
browser/mobile playback, durable storage, reconnect recovery, or static co-link
compatibility is established by this command. See `docs/moq-spike.md` for measured
results and cutover blockers.
