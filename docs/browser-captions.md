# Browser captions and OBS

The authoritative [streamer guide](../js/docs/src/content/docs/guides/start-streaming/captions.md#browser-captioner-and-obs-display)
covers the browser captioner, OBS display source, permissions, calibration, and
caption policy. See the [operator guide](../js/docs/src/content/docs/guides/installing/captions.md)
for node resources and the [protocol notes](../js/docs/src/content/docs/features-dev/captions.md)
for push authentication, live delivery, and archival formats.

## Building and serving

Run `make whisper-wasm` in the builder container. It downloads/caches Emscripten
3.1.74 under `.build/whisper-wasm` and builds whisper.cpp v1.8.2 revision
`4979e04f5dcaccb36057e059bbaed8a2f5288315`, matching the native engine's wrap.
The source archive is SHA-256 verified. The Dockerfile and builder image hash are
unchanged; the tradeoff is a first-build SDK download (about 328 MB), cached on
subsequent builds. The builder's Node 22.15+ strips the worker's TypeScript without
running workspace installs. `make app` and `make app-cached` include this target.

Generated WASM/JS/worker assets live under `js/app/assets/whisper/<revision>` and
are embedded by `app.AssetFiles`; generated artifacts are not checked in. Routes:

```text
/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/caption-whisper.wasm
/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/caption-whisper.mjs
/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/worker.js
/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/ggml-{tiny,base,small}-q5_1.bin
/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/ggml-silero-v5.1.2.bin
```

Model bytes come from `stt.ModelFiles`; builds without bundled models return
503 for model requests. Successful assets have immutable one-year cache headers.
The captioner document and `?deviceCaptions=1` browser go-live documents opt into
`Cross-Origin-Opener-Policy: same-origin` and
`Cross-Origin-Embedder-Policy: credentialless`. Preserve these headers through
reverse proxies. Assets also have `Cross-Origin-Resource-Policy: same-origin`.
Captioner OAuth uses redirects rather than popups on the isolated page.

## Browser smoke

With a node serving these assets and the native engine's real JFK sample:

```sh
node hack/whisper-wasm/smoke.mjs http://127.0.0.1:YOUR-PORT /path/to/jfk.wav
```

The script loads every quantized model in Chromium and transcribes the actual
11-second speech sample. It also renders the captioner and feeds simulated
interim/final websocket messages into the overlay. Output screenshots go to
`.build/captionobs-smoke`, created automatically by the script.

Observed in the builder's headless Chromium 149, four inference threads:

| Model | Processing time / audio time |
| ----- | ---------------------------: |
| tiny  |                        1.053 |
| base  |                        2.158 |
| small |                        3.893 |

All three correctly transcribed the “ask not” speech. **These measurements were
slower than realtime on this machine**; use the page's measurement and live-speed
warning rather than assuming browser captioning can keep up. The smoke used an
isolated asset adapter with the native engine's real model files, not a full
node/OAuth/live-ingest test.
