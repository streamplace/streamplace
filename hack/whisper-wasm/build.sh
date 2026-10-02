#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT="$PWD"
NODE="$(command -v node)"
REV=4979e04f5dcaccb36057e059bbaed8a2f5288315
EMSDK_VERSION=3.1.74
SOURCE_SHA256=ec8e33f4c838ed459ad0a23da255c0262f150a6f54839a17e504ddf1495305d5
CACHE="$ROOT/.build/whisper-wasm"
mkdir -p "$CACHE"
if [ ! -d "$CACHE/emsdk" ]; then
  git clone --depth 1 --branch "$EMSDK_VERSION" https://github.com/emscripten-core/emsdk.git "$CACHE/emsdk"
fi
test "$(git -C "$CACHE/emsdk" rev-parse HEAD)" = 3d6d8ee910466516a53e665b86458faa81dae9ba
if [ ! -f "$CACHE/emsdk/upstream/emscripten/emcc" ]; then
  "$CACHE/emsdk/emsdk" install "$EMSDK_VERSION"
fi
"$CACHE/emsdk/emsdk" activate "$EMSDK_VERSION"
source "$CACHE/emsdk/emsdk_env.sh"
if [ ! -f "$CACHE/source.tar.gz" ]; then
  curl --fail --location --retry 3 "https://codeload.github.com/ggml-org/whisper.cpp/tar.gz/$REV" -o "$CACHE/source.tar.gz"
fi
# Compare the digest directly: macOS's sha256sum takes no check list on
# stdin, and older macOS only has shasum.
sum=$(shasum -a 256 "$CACHE/source.tar.gz" 2>/dev/null || sha256sum "$CACHE/source.tar.gz")
if [ "${sum%% *}" != "$SOURCE_SHA256" ]; then
  echo "whisper.cpp source checksum mismatch: got ${sum%% *}, want $SOURCE_SHA256" >&2
  exit 1
fi
if [ ! -f "$CACHE/source/CMakeLists.txt" ]; then
  mkdir -p "$CACHE/source"
  tar -xz --strip-components=1 -C "$CACHE/source" -f "$CACHE/source.tar.gz"
fi
# The build cache is shared by host and container runs; a cached compiler
# launcher path from one environment may not exist in the other.
emcmake cmake -S "$ROOT/hack/whisper-wasm" -B "$CACHE/build" -G Ninja \
  -DCMAKE_BUILD_TYPE=Release -DWHISPER_SOURCE="$CACHE/source" -DGGML_CCACHE=OFF
cmake --build "$CACHE/build" --target caption-whisper --parallel 4
DEST="$ROOT/js/app/assets/whisper/$REV"
mkdir -p "$DEST"
cp "$CACHE/build/caption-whisper.mjs" "$CACHE/build/caption-whisper.wasm" "$DEST/"
"$NODE" hack/whisper-wasm/transpile-worker.mjs "$DEST"
printf '%s\n' "WASM artifacts: $DEST"
wc -c "$DEST"/*
