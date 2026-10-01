# Bundled Whisper speech recognition

The CPU-only whisper.cpp v1.8.2 subproject is pinned to commit
`4979e04f5dcaccb36057e059bbaed8a2f5288315` and its tarball SHA256 in
`subprojects/whisper.wrap`. ggml 0.9.4 is vendored in that upstream revision.
The packagefiles Meson build compiles sources directly, without CMake or host
architecture probes, and produces a PIC static archive for shared development
and static release dependencies (`streamplacedeps` and `whisper` pkg-config).

## Portability

x86-64 uses AVX2, FMA, and F16C, never `-march=native`. Upstream ggml's multiple
CPU variants require dynamic backend libraries; they cannot be statically
bundled by this implementation. Before entering that archive, `NewEngine`
checks CPU features and returns an error on unsupported machines. The node can
continue without automatic captions; model serving and license notices remain
available. arm64 uses its mandatory ARMv8 NEON baseline without optional
dot-product, i8mm, SVE, Metal, CUDA, Accelerate, BLAS, or OpenMP dependencies.

The source list and system definitions support the existing linux amd64/arm64,
darwin amd64/arm64, and Windows amd64 GNU cross files. Linux uses pthreads,
libm, libdl and libstdc++; Darwin uses pthreads, libm and libc++; Windows uses
the existing MinGW toolchain and its thread/C++ runtime. Cross-platform archive
configuration does not run a target executable.

## Assets and licenses

`make captions-assets` fetches multilingual `ggml-tiny-q5_1.bin`,
`ggml-base-q5_1.bin`, `ggml-small-q5_1.bin`, and `ggml-silero-v5.1.2.bin`.
Each download is SHA256-verified, cached under the user's cache directory in
`streamplace/captions/<sha256>`, and hard-linked (or copied across filesystems)
to ignored `pkg/stt/assets/` files. Meson configuration and `make dev` invoke
this downloader. These weights are embedded in the Go binary; no model files
or audio are committed. `stt.ModelFiles` exposes these filenames and
`licenses.txt` to browser-captioner serving code. Models are decoded once per
engine model; subsequent inference passes only PCM through cgo. Independent
pooled whisper states share model weights and retain their own Silero VAD.

`pkg/licenses/attributions.txt` contains notices for Streamplace, whisper.cpp,
ggml, OpenAI Whisper weights, ggml conversions, and Silero VAD. It is not a
claim to inventory all pre-existing dependencies. Public API:

- `streamplace --licenses`: prints the notices and exits before node startup.
- `GET /api/licenses`: unauthenticated `text/plain; charset=utf-8`, same notices.
- `stt.NewEngine(context.Context, *config.CLI) (stt.Engine, error)` implements
  the settled `pkg/stt/stt.go` contract.

## Benchmark and scheduler

Construction starts loading and benchmarking in a background goroutine. A
three-second synthetic voiced signal bypasses VAD, avoiding a spuriously cheap
silence benchmark. Weight loading is excluded from measured realtime factor.
Per-stream threads are `min(4, floor(logicalCPUs * CaptionsCPUBudget))`, at least
one. `Models()` reports zero factors until measurement; `Lease` waits for it
with cancellation. A failed model is excluded instead of offering a fake model.

Realtime reservations cost `threads * measuredRTF * 1.2` logical CPUs, including
20% headroom. Realtime models must also have RTF <= 1. Realtime leases share the
remaining capacity equally, downgrade immediately on overload, and upgrade only
when the larger model uses no more than 80% of their share. Explicit model names
remain pinned; admission fails with `ErrOverBudget` if that model cannot fit.
Non-realtime leases wait for spare thread capacity and receive the largest
available model, or their requested model. On nodes budgeted below one logical
CPU, they reserve that fractional budget and pace inference with cancellable
idle time instead of waiting forever for a whole CPU. Call `Lease.Model()` for every window
because realtime leases can change between calls. Release leases when finished.
Extra `ggml-*.bin` files in `CaptionsModelDir` are loaded and measured too.
Models are ordered by loaded network dimensions, not benchmark timing or
quantized file size. Silero-named files are excluded from the recognition catalog.

Recognition accepts 16kHz mono float32 PCM, language detection or a forced
language (regional tags use their primary language), and explicit prompt text.
Whisper's token timestamps are merged into words with subword continuations and
punctuation. Results include probabilities and window-relative offsets. Silero
skips speech-free windows before encoder inference. Context cancellation aborts
whisper computation, and state history does not leak across streams.

## Scoped verification

Run `make dev`, then `go test -count=1 ./pkg/stt/...` in the builder. During
native development, add `build-linux-amd64/meson-uninstalled` to
`PKG_CONFIG_PATH` if the new archive has not yet been installed.

The real-speech integration fixture is the public-domain excerpt of John F.
Kennedy's 1961 inaugural address from whisper.cpp `samples/jfk.wav`. Its source
revision and SHA256 are pinned by the asset downloader. It is downloaded for
tests only and not embedded. The word test recognizes “ask not what your
country”, checks word offsets/probabilities and language, and verifies silence
and cancellation. Scheduler tests use deterministic fake costs and check budget,
pinned models, hysteresis, batch waiting, and shutdown. The benchmark smoke
prints measured factors for all three bundled models.
Proxy regressions share admission with direct parent leases, release leases on
worker disconnect/engine shutdown, propagate inference cancellation, and reject
oversized frames. Real-model proxy smoke also recognizes that JFK excerpt
through the parent engine; workers do not need a second model load.

## Origin mastering

The node initializes one `MediaManager.STT` engine shared by in-process ingest,
sidecars, VOD jobs, and isolated workers. Workers lease and transcribe over a
private mode-0600 Unix socket; they never load a separate engine. Disconnecting
a worker releases its lease and cancels inference. Framed requests are bounded
to 16 MiB and recognition windows to 30 seconds. Origin recognition does not
depend on `SP_CAPTIONS`, which controls optional node sidecars.
Canonical sources publish into a private per-session hub, never directly into
the public caption hub. The same per-GoP metadata snapshot supplies both the
signed manifest and caption policy. Sources wait for the first snapshot before
decoding or leasing recognition, so an ingest-only or off policy never briefly
starts automatic recognition during startup.

The streaming MUXL signer requests text for each AV GoP, through the actual
next-keyframe boundary. It waits until finalized recognition covers that GoP's
end, or until GoP closure plus `SP_CAPTIONS_MASTER_DELAY`, whichever comes first.
Lossless byte queues block producers at 32 MiB instead of dropping media or
growing indefinitely; aborts discard retained bytes and unblock writers.
Late final words move into the next unsigned GoP; cross-boundary cues retain
their session-qualified ID and are clipped into each overlapping GoP. Text
tracks are declared lazily per source/language, starting at reserved ID 100,
above node-added AV renditions. Ingest takeover declares a new immutable track
and the old automatic track continues as gaps; pushed language tags are not
replaced by recognition hints.
Continuous audio completion feeds only AV tracks to its native decoder, whose
initial track configuration cannot change. Lazily declared caption tracks stay
in the original signed source bytes and in the completed archival segment.

All origin policies stamp GoPs with media start time, anchored to the first
fragment's arrival rather than time spent signing. An arrival/prediction drift
over one second reanchors the next GoP. Pushed wall-clock cues use the inverse
of that same mapping. Optional generic MUXL `SegmentTimeFn` leaves signing-time
stamps byte-identical when nil.

`place.stream.caption.pushCaptions` accepts the streamer's OAuth session or
`Authorization: Bearer <stream key>`. Stream-key pushes require an active
registered `place.stream.key`, apply the same stream allowance and ban checks
as WHIP, and cannot name a different streamer. Non-live streams return
`StreamNotLive`. Canonical pushes travel into the origin master (including
private worker control sockets), using its ingest-arrival/media clock to
translate wall-clock cue times. With canonical captions off, pushes instead
follow the signed policy's sidecar/local routing.
Push requests are limited to 2 MiB before binding. Each batch is validated
atomically against the lexicon limits (100 cues, 64-character IDs, 2000-character
text, BCP 47 language and ordered timestamps) before any cue is published.

## Live distribution and archival captions

Canonical captions reach every node, including the ingest node, only by reading
the text tracks in validated MUXL segments. `TextTrack.Language` is the BCP 47
tag (`und` when unknown); `TextTrack.Label` is `auto`, `ingest`, or `human`
(empty or unknown labels are treated as ingest). Empty text segments publish no
cues. Tracks can appear during the
session, and pieces of one cue crossing GoPs extend one hub/HLS cue. Cue times
use the signed segment start plus the difference between the cue's media time
and the reference AV decode time, never the receiving node's clock.

When canonical captions are off at the origin, or absent on a relay, the signed
`allowNodeCaptions` preference and `SP_CAPTIONS` gate sidecar recognition. Sidecar
recognition uses the same engine budget and continuous audio decoder as origin
mastering. A missing engine or refused lease leaves media playback unaffected.
An incoming canonical track or upstream sidecar stops competing recognition.
Upstream replay arriving before its first media segment stays private until the
signed policy is validated; canonical tracks take precedence over that replay.
Canonical-track presence is classified before accepting following sidecar frames;
discovered canonical tracks also remove competing sidecars from the live hub.

Sidecars travel on the existing segment-replication websocket, opt-in with
`captions=1` on `place.stream.live.subscribeSegments`: binary frames remain media,
and JSON text frames carry `place.stream.caption.sidecar#event` with segment-clock
cue times. Receivers derive the sidecar track ID from origin/source/language and
bind its author to the connected upstream's server DID, not its claimed author.
This keeps media and captions on the
same upstream connection; old peers do not request the capability and continue
receiving only binary frames. Websocket replication is the implemented origin
pull path; the Iroh replicator currently has no segment transport implementation.
Both locally authored and forwarded sidecars honor `allowNodeCaptions=false`.
The hub rejects cross-origin ID collisions, does not store interim cues, and
bounds track count and final history. Viewer subscriptions remain nonblocking.

Transcript writers begin with the first published segment. Only the origin
writes canonical transcripts through the streamer's stored OAuth session; a
node writes only its own sidecars to its server repo, never an upstream node's
records. Session end (including a return to preview) clears live captions and
starts an asynchronous final record flush; PDS retries cannot block media.
The newest canonical cue remains mutable across a periodic flush until another
cue follows, its end is older than one flush interval, or the session ends, so a
GoP boundary cannot truncate the recorded cue. Sidecar finals need no such hold.
Writers reconcile retained hub finals before every flush and before session
teardown, so a full viewer event buffer does not silently lose transcript words.
Their deduplication state is limited to the retained replay window.
Encoded records persist in `DataDir/captions/transcripts` until delivery succeeds.
PDS rate-limit reset times apply to final flushes and recovered records too;
records that outlive the shutdown deadline resume with their original record
keys on the next node startup. The outbox contains no OAuth credentials.
Node shutdown stops caption admission, drains accepted canonical segments, and
waits for workers and all current or already-finishing transcript sessions
before closing the speech engine.

Live-derived VOD metafiles retain text-track metadata and byte ranges, including
each segment's language/source configuration and containing-GoP reference clock.
Reused numeric IDs after a reconnect cannot relabel earlier captions. New indexes
store an explicit containing-GoP reference, rather than infer it from text-byte
placement. Archives must obey MUXL's ascending numeric track-ID order within each
GoP; the indexer rejects noncanonical ordering without rewriting signed bytes.
`publishDraft` publishes their `place.stream.media.track` records alongside AV
tracks, rather than exposing tracks while the video is still a draft. VOD
caption lookup places text segments on the video's AV timeline, including late
track declarations and reconnects, and merges them with indexed transcripts.
A matching record copy of the streamer's mastered text is omitted; imports,
sidecars, and different authored text remain available.

Imported SRT/WebVTT and human transcript tracks reconstruct cues only at explicit
silence gaps, preserving long cues, adjacent cue boundaries, and authored line
breaks. Imports encode a 1 ms boundary gap between otherwise adjacent cues by
shortening the preceding final word (or moving the next start 1 ms when that word
is already only 1 ms). Automatic and ingest transcripts retain broadcast display
layout. VTT, SRT, JSON, VOD HLS and overlays all use this same record provider.
