<p align="center">
  <img src="streamplace-logo.svg" width="200" alt="Streamplace logo">
</p>
<h2 align="center">Streamplace</h2>
<p align="center"><strong>Solving Video for Everybody Forever</strong></p>

Welcome friends! This is the codebase for Streamplace, live video built on the
[AT Protocol](https://atproto.com), which is the same protocol that powers Bluesky.

Get started fast:

- **Web: [stream.place](https://stream.place)**
- **Download: [stream.place/download](https://stream.place/download)**
- **Docs: [stream.place/docs](https://stream.place/docs)**

## Development Resources

The server is written in Go (`pkg/`, `cmd/`) and the app is a
[React Native](https://reactnative.dev/) application in TypeScript (`js/`).
The `place.stream.*` Lexicons live in [`lexicons/`](lexicons/).

You don't _need_ to understand the AT Protocol to work with Streamplace, but
it helps quite a lot. Learn more at [atproto.com](https://atproto.com/guides/overview).

Good places to start:

- [Quick start: streaming](https://stream.place/docs/guides/start-streaming/quick-start)
- [Development setup](https://stream.place/docs/guides/start-contributing/streamplace-dev-setup)
- [Self-hosting](https://stream.place/docs/guides/installing/downloading-streamplace)
- [API reference](https://stream.place/docs/lex-reference/place-stream-defs)

## Shadow-testing native RTMP ingest with Mist

Add `--duplicate-mist-test` (or `SP_DUPLICATE_MIST_TEST=true`) to an existing
Mist-backed RTMPS configuration. It requires `--secure` and
`--rtmp-server-addon` pointing to Mist, and applies to the
`--rtmps-addon-addr` listener—not the native `--rtmps-addr` listener.
Mist remains the only server responding to the encoder.

After TLS termination, the client byte stream is also replayed in a separate
process through the native RTMP reader, relay, GStreamer ingest, segment
signer, and media/signature verifier. The worker ignores the supplied stream
key for signing and uses a fresh ephemeral key. Its output is discarded:
no duplicate live segments, recordings, archive writes, or stream state.

Look for `duplicate-mist-test segment verified` progress (first segment, then
every 30 seconds) and `segments_verified` at disconnect. Parser, ingest,
validation, worker-exit, and queue-overflow failures log
`duplicate-mist-test failed` with connection identifiers. Protocol error
details that could contain stream keys are redacted.

The shadow has an 8 MiB per-connection buffer and a cap of 16 workers per
addon listener, including draining workers. At capacity, new connections
forward only to Mist and log a shadow failure. Shadow input idle for 10
seconds is aborted without closing the primary connection. Buffer overflow
also kills only the shadow; Mist continues unchanged. On disconnect it
drains the native pipeline through EOS and verifies the final segment,
with a 30-second worker-exit deadline. Node shutdown kills/reaps workers.
This is opt-in diagnostic work with additional CPU and memory cost, up to
128 MiB of tee buffers plus native worker memory. Plain RTMP inside TLS is
supported; RTMPE's separately negotiated encryption cannot be passively replayed.

The backend end-to-end regression is `TestDuplicateMistEndToEnd` in
`pkg/cmd`; it exercises a real TLS publisher and the isolated native ingest
through verified segments beyond the former few-second failure interval,
including all final segments after paced and unpaced publisher EOF.
No client UI or platform-specific iOS/Android/Web behavior changes.

## Contributions

Check for existing [issues](https://github.com/streamplace/streamplace/issues)
before filing a new one, and open an issue for discussion before submitting a
large PR. Questions and chat are welcome on
[Discord](https://discord.stream.place).
