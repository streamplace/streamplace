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
| `SP_CAPTIONS_MASTER_DELAY` | `--captions-master-delay` | `1.5s`                  | Maximum time after a GoP closes that the origin may hold it waiting for captions before signing its canonical MUXL segment. Late final words move into the next unsigned GoP.                                                        |

For example:

```ini
SP_CAPTIONS=true
SP_CAPTIONS_CPU_BUDGET=0.5
SP_CAPTIONS_MASTER_DELAY=1.5s
# Optional extra models:
# SP_CAPTIONS_MODEL_DIR=/var/lib/streamplace/whisper-models
```

The flag names, descriptions, and defaults are defined in
[`pkg/config/config.go`](https://github.com/streamplace/streamplace/blob/main/pkg/config/config.go).
Budget validation and extra-model discovery are in
[`pkg/stt/engine.go`](https://github.com/streamplace/streamplace/blob/main/pkg/stt/engine.go).
This budget is speech-engine admission and scheduling, not an operating-system
CPU quota for the entire node. Isolated ingest workers own their own engines;
account for that when sizing a multi-worker deployment.

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

The signer waits until recognition covers the GoP end or until GoP closure
plus `SP_CAPTIONS_MASTER_DELAY`, whichever comes first. Buffering separates
that wait from ingest. A larger delay gives recognition more time to arrive
in the matching segment but increases origin latency **by up to that delay**;
a smaller delay releases media sooner but can carry late words into the next
unsigned segment. This is not the total player latency: encoding, segment
length, network, and playback buffering also contribute. See
[`captions_master.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_master.go)
for the hold and late-cue behavior.

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
