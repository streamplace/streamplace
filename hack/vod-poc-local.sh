#!/usr/bin/env bash
#
# Serve the VOD time-to-first-frame proof of concept (js/vod-poc), then run
# its Playwright flow and the vod-ttff benchmark against it.
#
#   hack/vod-poc-local.sh [--serve] [-- <extra playwright args>]
#
# --serve prints the watch URLs and keeps serving instead of testing.
# Needs a streamplace binary (`make dev`) and ffmpeg; the media is generated
# once into js/vod-poc/media (or $VOD_POC_MEDIA).
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
# shellcheck source=hack/lib/e2e-harness.sh
. hack/lib/e2e-harness.sh

serve=
if [ "${1:-}" = "--serve" ]; then
  serve=1
  shift
fi
[ "${1:-}" != "--" ] || shift

media="${VOD_POC_MEDIA:-$REPO/js/vod-poc/media}"
[ -f "$media/renditions.json" ] || js/vod-poc/prepare-media.sh "$media"
pnpm --filter @streamplace/vod-poc build >/dev/null

out="$(mktemp)"
"$BUILDDIR/streamplace" vod-poc --media "$media" --web js/vod-poc \
  ${VOD_POC_HTTP_ADDR:+--http-addr "$VOD_POC_HTTP_ADDR"} >"$out" &
pid=$!
# The server dies of the kill; that is not this script's failure under set -e.
trap 'kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true; rm -f "$out"' EXIT
for _ in $(seq 120); do
  grep -q '^VOD_POC_URLS=' "$out" && break
  kill -0 "$pid" || { echo "vod-poc exited before serving"; exit 1; }
  sleep 0.5
done
VOD_POC_URLS="$(sed -n 's/^VOD_POC_URLS=//p' "$out")"
[ -n "$VOD_POC_URLS" ] || { echo "vod-poc did not start"; exit 1; }
export VOD_POC_URLS

if [ -n "$serve" ]; then
  echo "serving: $VOD_POC_URLS"
  wait "$pid"
  exit
fi
cd "$REPO/js/e2e-web"
VOD_TTFF_VIDEOS="$VOD_POC_URLS" pnpm exec playwright test \
  --config playwright.bench.config.ts "$@"
