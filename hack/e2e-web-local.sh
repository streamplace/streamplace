#!/usr/bin/env bash
#
# Run the headless-web e2e suite (Playwright) against a self-contained
# `streamplace e2e` harness — the web counterpart to hack/e2e-local.sh.
# This is also what CI runs (via `make e2e-web`) after `make linux-amd64`.
#
#   hack/e2e-web-local.sh [-- <extra playwright args>]
#
# Prerequisites:
#   * A streamplace binary that embeds the web app: `make linux-amd64` (or
#     `make dev` for the dev wrapper). Override the binary with SP_E2E_BIN.
#     The suite's global-setup points the app at the harness node via
#     Settings -> Advanced, so a production-pointed bundle works too.
#   * Playwright + its chromium browser installed in js/e2e-web
#     (pnpm --filter @streamplace/e2e-web install-browser).
#
# On a dev host without the cgo runtime, run this inside the build container.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

# --- locate the harness binary (build-<os>-<arch>/streamplace) -------------
case "$(uname)-$(uname -m)" in
  Linux-x86_64)  BUILDDIR=build-linux-amd64 ;;
  Linux-aarch64) BUILDDIR=build-linux-arm64 ;;
  Darwin-arm64)  BUILDDIR=build-darwin-arm64 ;;
  Darwin-x86_64) BUILDDIR=build-darwin-amd64 ;;
  *) echo "unsupported host $(uname)-$(uname -m)"; exit 1 ;;
esac
SP_E2E_BIN="${SP_E2E_BIN:-$BUILDDIR/streamplace}"
if [ ! -x "$SP_E2E_BIN" ]; then
  echo "harness binary $SP_E2E_BIN missing — run 'make linux-amd64' (or 'make dev') first"
  exit 1
fi

# --- start the harness, capture its env, and always clean it up ------------
ENVFILE="$(mktemp)"
LOGFILE="${SP_E2E_HARNESS_LOG:-$(mktemp)}"
echo "starting e2e harness…"
"$SP_E2E_BIN" e2e > "$ENVFILE" 2> "$LOGFILE" &
HARNESS_PID=$!
cleanup() {
  kill "$HARNESS_PID" 2>/dev/null || true
  pkill -P "$HARNESS_PID" 2>/dev/null || true
  rm -f "$ENVFILE"
}
trap cleanup EXIT

for _ in $(seq 1 90); do grep -q SERVER_URL "$ENVFILE" && break; sleep 2; done
if ! grep -q SERVER_URL "$ENVFILE"; then
  echo "harness failed to start; log:"; tail -50 "$LOGFILE"; exit 1
fi
# shellcheck disable=SC1090
. "$ENVFILE"
export SERVER_URL ACCOUNT_HANDLE ACCOUNT_DID
echo "harness up: SERVER_URL=$SERVER_URL ACCOUNT_HANDLE=$ACCOUNT_HANDLE"

# --- run the flows ---------------------------------------------------------
cd "$REPO/js/e2e-web"
if ! pnpm exec playwright test "$@"; then
  echo "--- playwright failed; last 100 lines of harness log ---"
  tail -100 "$LOGFILE"
  exit 1
fi
