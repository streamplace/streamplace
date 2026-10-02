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
libm, libdl and libstdc++; Darwin uses pthreads, libm and libc++. The
darwin-amd64 release now requires macOS 10.15 or newer because ggml uses
`std::filesystem`. The osxcross profile sets that deployment target for C,
C++, Objective-C and their linkers; `make darwin-amd64` also applies it to
cgo and exports `MACOSX_DEPLOYMENT_TARGET` for the other native toolchains.
Windows uses the MinGW POSIX-thread compiler variants so `std::mutex` and
`std::thread` work, and links libstdc++, libgcc and winpthread statically;
no additional runtime DLLs are required. Whisper sets the Windows 8 API level
(`_WIN32_WINNT=0x0602`) for `SetThreadInformation`. Its scoped compatibility
header supplies the standard power-throttling declarations missing from
MinGW-w64 8's older SDK headers, without disabling ggml's API call.
Cross-platform archive configuration does not run a target executable.

The emscripten build of the WASM captioner is not committed: `make app` runs
`make whisper-wasm`, which builds it with the pinned emsdk on Linux or macOS
(including the iOS runner) and is incremental after the first run. The
whisper.cpp source tarball is checked by comparing its SHA-256 digest, since
macOS's `sha256sum` reads no check list from stdin and older macOS only has
`shasum`.

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
one. `Lease` waits for measurement with cancellation. A failed model is excluded
instead of offering a fake model.

Realtime reservations cost `threads * measuredRTF * 1.2` logical CPUs, including
20% headroom. Realtime models must also have RTF <= 1. Realtime leases share the
remaining capacity equally, downgrade immediately on overload, and upgrade only
when the larger model uses no more than 80% of their share. Admission is
live/realtime-only: there are no batch/VOD jobs or explicit model pins.
Call `Lease.Model()` for every window because leases can change between calls.
Release leases when finished.
Extra `ggml-*.bin` files in `CaptionsModelDir` are loaded and measured too.
Models are ordered by loaded network dimensions, not benchmark timing or
quantized file size. Silero-named files are excluded from the recognition catalog.

Recognition accepts 16kHz mono float32 PCM, language detection or a forced
language (regional tags use their primary language), and explicit prompt text.
Whisper's token timestamps are merged into words with subword continuations and
punctuation. Results include probabilities and window-relative offsets. Silero
skips speech-free windows before encoder inference. Context cancellation aborts
whisper computation, and state history does not leak across streams.

whisper.cpp and ggml print through a log callback instead of stderr. What a
recognition call (or a model load) prints becomes one debug log line, `whisper
output=...`, which appears at `-v=4`.

## Scoped verification

Run `make dev`, then `go test -count=1 ./pkg/stt/...` in the builder. During
native development, add `build-linux-amd64/meson-uninstalled` to
`PKG_CONFIG_PATH` if the new archive has not yet been installed.

For native-only cross checks, configure the release directory with
`make meson-setup-static BUILDDIR=build-windows-amd64 MESON_SETUP_OPTS="--cross-file util/windows-amd64-gnu.ini"`
(substitute `build-darwin-amd64` and `util/osxcross-darwin-amd64.ini` for macOS),
then run `meson compile -C build-windows-amd64 whisper`. These commands do not
compile the Go application. After editing the whisper packagefiles in an
already-extracted checkout, run `meson subprojects packagefiles --apply whisper`
and `meson setup --reconfigure <build-directory>` before compiling.

The real-speech integration fixture is the public-domain excerpt of John F.
Kennedy's 1961 inaugural address from whisper.cpp `samples/jfk.wav`. Its source
revision and SHA256 are pinned by the asset downloader. It is downloaded for
tests only and not embedded. The word test recognizes “ask not what your
country”, checks word offsets/probabilities and language, and verifies silence
and cancellation. Scheduler tests use deterministic fake costs and check budget,
model selection, hysteresis, and shutdown. The benchmark smoke prints measured
factors for all three bundled models.
Proxy regressions share admission with direct parent leases, release leases on
worker disconnect/engine shutdown, propagate inference cancellation, and reject
oversized frames. Real-model proxy smoke also recognizes that JFK excerpt
through the parent engine; workers do not need a second model load.

## Operational and protocol documentation

The published [operator guide](../js/docs/src/content/docs/guides/installing/captions.md)
is authoritative for engine sharing, caption mastering, resource configuration,
and shutdown/delivery guarantees. The [protocol notes](../js/docs/src/content/docs/features-dev/captions.md)
cover signed MUXL text, live distribution, transcript records, and VOD indexing.
For caption policy, browser/OBS setup, and push authentication, use the
[streamer guide](../js/docs/src/content/docs/guides/start-streaming/captions.md).
