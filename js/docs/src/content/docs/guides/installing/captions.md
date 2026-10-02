---
title: Operating a node with captions
description: Configure speech recognition resources, mastering latency, and caption relaying.
sidebar:
  order: 40
---

Streamplace can master automatic captions into an origin stream and generate
separate accessibility captions on nodes relaying streams without a canonical
caption track. Streamer preferences determine which behavior is allowed. See
[the streamer guide](/docs/guides/start-streaming/captions/) for dashboard controls.

## Configuration

Set environment variables in your node's service configuration; packaged Linux
installs use `/etc/streamplace/streamplace.env` as described in
[Downloading Streamplace](/docs/guides/installing/downloading-streamplace/).
The equivalent CLI flags are listed below.

| Environment variable       | CLI flag                  | Default                 | Effect                                                                                                                                                                                                                               |
| -------------------------- | ------------------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `SP_CAPTIONS`              | `--captions`              | `true`                  | Enables optional node-generated sidecars when the streamer's policy allows them. It does not disable an origin streamer's requested canonical automatic captions, or pass-through of existing captions.                              |
| `SP_CAPTIONS_CPU_BUDGET`   | `--captions-cpu-budget`   | `0.5`                   | Fraction of logical CPUs available to the speech-recognition engine across streams. Must be greater than zero and at most one. The scheduler measures models and chooses the largest model that fits each stream's available budget. |
| `SP_CAPTIONS_MODEL_DIR`    | `--captions-model-dir`    | Unset (no extra models) | Directory of additional `ggml-*.bin` Whisper models, offered alongside bundled models. Models are loaded and benchmarked; Silero-named files are not recognition models.                                                             |
| `SP_CAPTIONS_MASTER_DELAY` | `--captions-master-delay` | `10s`                   | Maximum time after a GoP closes that its **recorded** copy waits for speech recognition to cover it before the captions are laid out for the recording. Live segments are never held for captions.                                   |

For example:

```ini
SP_CAPTIONS=true
SP_CAPTIONS_CPU_BUDGET=0.5
SP_CAPTIONS_MASTER_DELAY=10s
# Optional extra models:
# SP_CAPTIONS_MODEL_DIR=/var/lib/streamplace/whisper-models
```

The flag names, descriptions, and defaults are defined in
[`pkg/config/config.go`](https://github.com/streamplace/streamplace/blob/main/pkg/config/config.go).
Budget validation and extra-model discovery are in
[`pkg/stt/engine.go`](https://github.com/streamplace/streamplace/blob/main/pkg/stt/engine.go).
This budget is speech-engine admission and scheduling, not an operating-system
CPU quota for the entire node. Isolated ingest workers share the parent node's
engine and CPU budget through a private mode-0600 Unix-socket proxy; they do not
load separate engines. Disconnecting a worker releases its lease and cancels
inference. Proxy frames are bounded to 16 MiB and recognition windows to 30 seconds.

## Bundled models and CPU requirements

The CPU-only whisper.cpp engine bundles multilingual, quantized **tiny**,
**base**, and **small** Whisper models (`ggml-{tiny,base,small}-q5_1.bin`) and a
Silero voice-activity detector. No GPU is required. Source builds fetch verified
assets with `make captions-assets`; model weights are embedded in the binary.

- **x86-64** requires **AVX2, FMA, and F16C**. The engine checks these features
  before running the backend. An unsupported CPU leaves automatic recognition
  unavailable rather than executing unsupported instructions.
- **arm64** uses the ARMv8 **NEON** baseline, without requiring optional
  dot-product, i8mm, SVE, or GPU backends.

Startup loads and benchmarks models in the background. The scheduler shares the
configured capacity across streams and may downgrade a realtime stream's
model as load grows. It includes headroom rather than assuming every machine
can run the small model live. An unavailable engine or refused recognition
lease leaves media playback working, but no automatic captions are produced
for that path. Supplied canonical captions still work without recognition.
Admission is live/realtime-only, without batch/VOD recognition or explicit model
pins. See the [native development notes](https://github.com/streamplace/streamplace/blob/main/docs/captions-whisper.md#benchmark-and-scheduler)
for thread costs and upgrade/downgrade hysteresis.

The bundled catalog and scheduler are in
[`engine.go`](https://github.com/streamplace/streamplace/blob/main/pkg/stt/engine.go),
the x86 feature check is in
[`bridge.c`](https://github.com/streamplace/streamplace/blob/main/pkg/stt/bridge.c),
and the CPU-only architecture build is in
[`the Whisper Meson configuration`](https://github.com/streamplace/streamplace/blob/main/subprojects/packagefiles/whisper/meson.build).

## Origin mastering and latency

With canonical `auto`, the origin recognizes the ingest audio and attaches
WebVTT text tracks **before signing** each MUXL segment. CEA-608/708 ingest
captions or pushed captions take precedence when supplied. With canonical
`ingest`, it only masters supplied captions; with canonical `off`, no canonical
caption text is attached.

Automatic captions are published phrase by phrase: words are final as soon as
two recognition passes agree on them, or once two seconds of audio follow
them. With the bundled models a pass takes one to three seconds on a typical
server, so captions usually trail speech by two to five seconds.

Live segments are signed and sent as soon as the ingest tap has parsed them;
the origin never holds video or audio for speech recognition. A word recognized
after its segment was signed is shown live in the first unsigned segment, after
the previous caption has had its own reading time, so viewers see captions a few
seconds behind the speech.

When the origin records the stream to S3, the recording gets captions in the
segment where they were spoken. A second, archival layout of each segment waits
until recognition covers it, or until GoP closure plus
`SP_CAPTIONS_MASTER_DELAY`, whichever comes first; with pushed captions, which
give no coverage signal, it waits the full delay. The recorded segment is the
live one with only its text runs replaced, re-signed with the streamer's key; its
audio and video bytes are unchanged. A larger delay therefore costs no live
latency, only memory for segments awaiting upload. Words that arrive after the
delay are recorded like late live captions. Isolated ingest workers send their
archival text to the main process before their stream ends. See
[`captions_master.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_master.go)
for both layouts and
[`captions_archive.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_archive.go)
for the recorded copy.
Sources wait for the first signed policy snapshot before recognition admission,
so ingest-only/off never briefly starts automatic recognition. Lossless byte
queues block at 32 MiB instead of dropping media; aborts discard retained bytes
and unblock writers.

## Relaying and sidecar syndication

Every node, including the origin, reads canonical captions from **validated
MUXL text tracks**. Relays pass those captions through; they do not independently
recognize speech over an incoming canonical track.

If canonical captions are absent, a node may recognize speech into a
**`sidecar`** track when both `SP_CAPTIONS=true` and the streamer's
**`allowNodeCaptions`** preference permit it. That preference defaults to true
and travels in the signed metadata. A received upstream sidecar is forwarded
instead of starting competing recognition. Canonical captions take precedence
if they later appear.

Sidecars are syndicated on the existing segment-replication websocket: peers
opt in with `captions=1` on `place.stream.live.subscribeSegments`. Media stays
in binary frames; sidecar events use JSON text frames. The generating node
remains the author when another node forwards its captions. Peers that do not
request caption syndication keep receiving media-only frames.

`allowNodeCaptions=false` prevents node-generated sidecars and their
syndication, including forwarding upstream sidecars. It does **not** block
canonical captions requested or supplied by the streamer. Turning
`SP_CAPTIONS=false` disables this node's optional recognition, not caption
pass-through or origin canonical recognition.

Canonical transcript records are written by the origin to the streamer's repo
using the stored streamer OAuth session. A node publishes only its own sidecar
transcripts to its server repo, not copies attributed to itself of another
node's records. See
[the transcript reference](/docs/lex-reference/caption/place-stream-caption-transcript/).

Routing and policy checks are implemented in
[`pkg/captions/policy.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/policy.go)
and [`captions_distribution.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_distribution.go).
For wire formats, see [the protocol notes](/docs/features-dev/captions/).

## Transcript delivery and shutdown

Writers begin with the first published segment and resolve the newest indexed
livestream record for each caption batch, so changing chapters during ingest
updates the transcript subject. A record created before the encoder starts is
valid unless its `endedAt` precedes session `mediaStart`; words wait for a
current record rather than being attributed to an already ended livestream.
Session end (including return to preview) clears live captions and starts an
asynchronous final record flush; PDS retries never block media. Writers retain
the newest canonical cue across
periodic flushes until another cue follows, its end is older than one flush
interval, or the session ends, preventing GoP boundaries from truncating records.
Sidecar finals need no such hold. Before flush and teardown, retained hub finals
are reconciled so an overflowing event buffer can recover transcript words.
Deduplication is bounded to the replay window.

Unpublished batches are queued **in memory**, capped and retried per streamer,
honoring PDS rate-limit reset times. Graceful shutdown flushes within its timeout,
after stopping admission and draining accepted canonical segments, before closing
the speech engine. A crash or restart during a PDS outage loses unpublished
transcript batches; canonical text remains archived in signed MUXL. There is no
durable transcript outbox or restart replay.

## License notices

Run:

```sh
streamplace --licenses
```

This prints the bundled attribution notices and exits before node startup.
A running node serves the same notices without authentication at
**`GET /api/licenses`**, as `text/plain; charset=utf-8`. The notices cover
Streamplace and the bundled speech components and weights; they are not an
inventory of every existing dependency.

These interfaces are implemented in
[`pkg/cmd/streamplace.go`](https://github.com/streamplace/streamplace/blob/main/pkg/cmd/streamplace.go),
[`pkg/api/api.go`](https://github.com/streamplace/streamplace/blob/main/pkg/api/api.go),
and [`pkg/licenses/licenses.go`](https://github.com/streamplace/streamplace/blob/main/pkg/licenses/licenses.go).
