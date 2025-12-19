#!/usr/bin/env bash
set -Eeuo pipefail

trap 'echo "error: ${BASH_SOURCE[0]}:${LINENO}" >&2' ERR

INPUT_FILE=${1:-}
if [[ -z "${INPUT_FILE}" ]]; then
  echo "usage: $(basename "$0") /path/to/input.mp4 [http_internal_addr] [public_http_base]" >&2
  echo "example: $(basename "$0") /home/user/streamplace/drawn.together.s01e01.hottub_000.mkv 127.0.0.1:39090 http://127.0.0.1:38080" >&2
  exit 2
fi

HTTP_INTERNAL_ADDR=${2:-127.0.0.1:39090}
PUBLIC_HTTP_BASE=${3:-http://127.0.0.1:38080}

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg not found in PATH" >&2
  exit 1
fi

if [[ ! -x "./build-linux-amd64/streamplace" ]]; then
  echo "missing executable: ./build-linux-amd64/streamplace" >&2
  exit 1
fi

if command -v curl >/dev/null 2>&1; then
  if ! curl -fsS --max-time 2 "http://${HTTP_INTERNAL_ADDR}/healthz" >/dev/null; then
    echo "internal Streamplace server not reachable at http://${HTTP_INTERNAL_ADDR}/healthz" >&2
    echo "start the main server (./build-linux-amd64/streamplace ...) and ensure --http-internal-addr matches ${HTTP_INTERNAL_ADDR}" >&2
    exit 1
  fi
fi

# Generate a fresh stream key (multibase private key) + derived did:key
read -r STREAM_KEY STREAM_DID < <(
  ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
  TMP_GO="$(mktemp -t streamplace-keygen.XXXXXX.go)"
  cleanup() { rm -f "${TMP_GO}"; }
  trap cleanup RETURN
  cat <<'EOF' >"${TMP_GO}"
package main

import (
  "fmt"
  "stream.place/streamplace/pkg/crypto/spkey"
)

func main() {
  priv, pub, err := spkey.GenerateStreamKey()
  if err != nil {
    panic(err)
  }
  // IMPORTANT: print ONLY priv.Multibase() as the stream key.
  // If you append DID bytes, Streamplace will treat it as a Bluesky DID and try to resolve it.
  fmt.Printf("%s %s\n", priv.Multibase(), pub.DIDKey())
}
EOF
  (
    cd -- "${ROOT_DIR}"
    go run "${TMP_GO}"
  )
)

if [[ -z "${STREAM_KEY}" || -z "${STREAM_DID}" ]]; then
  echo "failed to generate stream key" >&2
  exit 1
fi

echo "stream_key=${STREAM_KEY}" >&2
echo "did=${STREAM_DID}" >&2
echo "HLS master (once segments exist):" >&2
echo "  ${PUBLIC_HTTP_BASE}/api/playback/${STREAM_DID}/hls/index.m3u8" >&2
echo "" >&2

echo "Starting MKV ingest -> http://${HTTP_INTERNAL_ADDR}/live/<stream_key>" >&2
echo "Press Ctrl-C to stop." >&2

# Convert MP4 -> MKV (H264 + AAC) and send to streamplace internal ingest.
# MKVIngest expects matroska with H264 video + AAC audio.
# Use -re to read input at native frame rate (real-time).
# Video is copied, audio is transcoded to AAC (required by MKVIngest pipeline).
ffmpeg -re -stream_loop -1 \
  -i "${INPUT_FILE}" \
  -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -g 60 -keyint_min 60 -sc_threshold 0 \
  -c:a aac -ar 48000 -b:a 128k \
  -f matroska - \
| ./build-linux-amd64/streamplace live --http-internal-addr="${HTTP_INTERNAL_ADDR}" "${STREAM_KEY}"

