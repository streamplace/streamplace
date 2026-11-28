#!/usr/bin/env bash
#
# Run the headless-web e2e suite (Playwright) against a self-contained
# `streamplace e2e` harness — the web counterpart to hack/e2e-local.sh.
#
#   hack/e2e-web-local.sh [--edge] [--upstream-station] [-- <extra playwright args>]
#
# Prerequisites:
#   * A streamplace binary with the embedded web bundle (`make dev`).
#     EXPO_PUBLIC_WEB_TRY_LOCAL is *not* required: the Playwright global
#     setup drives Settings -> Advanced and points the app at the harness
#     node itself (the web counterpart of .maestro/setup/server-setup.yaml), so
#     a plain `make dev` build works. Building with
#     EXPO_PUBLIC_WEB_TRY_LOCAL=true only changes the app's compile-time
#     default (window origin instead of https://stream.place).
#   * Playwright + its chromium browser installed in js/e2e-web
#     (pnpm --filter @streamplace/e2e-web install-browser; needs root for
#     --with-deps, so run it inside the container).
#   * For the OAuth flow, the harness's HTTPS mode: see hack/lib/e2e-harness.sh.
#
# --edge puts js/edge's worker (wrangler dev) in front of the harness node and
# points the suite at the worker, the topology a broadcaster's apex hostname
# runs. Requires the worker's dependencies (`pnpm install`) and a free port
# (E2E_EDGE_PORT, default 18787).
#
# On a dev host without the cgo runtime, run this inside the build container.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

while [ $# -gt 0 ]; do
  case "$1" in
    --upstream-station)
      export SP_E2E_UPSTREAM_STATION=true
      shift
      ;;
    --edge)
      E2E_EDGE=true
      shift
      ;;
    *) break ;;
  esac
done
[ "${1:-}" != "--" ] || shift
# shellcheck source=hack/lib/e2e-harness.sh
. hack/lib/e2e-harness.sh

# --- edge worker -----------------------------------------------------------
EDGE_PORT="${E2E_EDGE_PORT:-18787}"
EDGE_PID=

edge_start() {
  local node_url="$SERVER_URL"
  local log
  if [ -n "${E2E_EDGE_LOG:-}" ]; then
    log="$E2E_EDGE_LOG"
  else
    log="$(mktemp)"
  fi
  if command -v ss >/dev/null 2>&1 && ss -ltn | grep -q ":$EDGE_PORT "; then
    echo "port $EDGE_PORT is in use; set E2E_EDGE_PORT" >&2
    exit 1
  fi
  ( cd "$REPO/js/edge" && node scripts/copy-assets.mjs ) || exit 1
  export WRANGLER_SEND_METRICS=false
  (
    cd "$REPO/js/edge"
    exec setsid pnpm exec wrangler dev --ip 127.0.0.1 --port "$EDGE_PORT" \
      --var "UPSTREAM_SERVER__0:$node_url"
  ) > "$log" 2>&1 &
  EDGE_PID=$!
  # e2e_harness_start installed its own trap; this one stops both.
  trap 'edge_stop; e2e_harness_stop' EXIT INT TERM

  local i
  for i in $(seq 1 60); do
    if curl -fsS -o /dev/null "http://127.0.0.1:$EDGE_PORT/" 2>/dev/null; then
      break
    fi
    sleep 1
  done
  if ! curl -fsS -o /dev/null "http://127.0.0.1:$EDGE_PORT/" 2>/dev/null; then
    echo "edge worker did not start; log:" >&2
    tail -30 "$log" >&2
    exit 1
  fi
  SERVER_URL="http://127.0.0.1:$EDGE_PORT"
  export SERVER_URL
  echo "edge up: SERVER_URL=$SERVER_URL upstream=$node_url"
}

edge_stop() {
  [ -n "${EDGE_PID:-}" ] || return 0
  kill -- -"$EDGE_PID" 2>/dev/null || kill "$EDGE_PID" 2>/dev/null || true
  wait "$EDGE_PID" 2>/dev/null || true
  EDGE_PID=
}

e2e_harness_start
if [ "${E2E_EDGE:-}" = true ]; then
  edge_start
fi
if [ "${E2E_UPSTREAM_STATION:-}" = true ] && [ $# -eq 0 ]; then
  set -- upstream-station.spec.ts
fi
if [ "${E2E_EDGE:-}" = true ] && [ $# -eq 0 ]; then
  # Sign-in flows drive OAuth through the harness's HTTPS node hostnames, so
  # they reach the node directly and cannot run through the worker. The
  # player-viewport flow opts into the Vite beta frontend with sp_web_beta,
  # which only a node serves; the worker serves the app build.
  EDGE_EXCLUDE='about-link|auto-publish-vods|bootstrap-branding|branding-favicons|chat-facets|chat-popout|chat-timezone|desktop|invalid-handle|login-typeahead|not-found|oauth-login|player-viewport|prelive-playback|pronouns|stream|upstream-station'
  set -- $(cd "$REPO/js/e2e-web/flows" && ls *.spec.ts | grep -Ev "^(${EDGE_EXCLUDE})\.spec\.ts$")
fi

# --- run the flows ---------------------------------------------------------
# No `exec` here: that would replace this shell and discard the EXIT trap,
# leaving the harness and its children running after a successful run.
cd "$REPO/js/e2e-web"
pnpm exec playwright test "$@"
