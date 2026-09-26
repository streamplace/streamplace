#!/usr/bin/env bash
#
# Run the headless-web e2e suite (Playwright) against a self-contained
# `streamplace e2e` harness — the web counterpart to hack/e2e-local.sh.
#
#   hack/e2e-web-local.sh [-- <extra playwright args>]
#
# Prerequisites:
#   * A streamplace binary with the embedded web bundle (`make dev`).
#     EXPO_PUBLIC_WEB_TRY_LOCAL is *not* required: the Playwright global
#     setup drives Settings -> Advanced and points the app at the harness
#     node itself (the web counterpart of .maestro/00-server-setup.yaml), so
#     a plain `make dev` build works. Building with
#     EXPO_PUBLIC_WEB_TRY_LOCAL=true only changes the app's compile-time
#     default (window origin instead of https://stream.place).
#   * Playwright + its chromium browser installed in js/e2e-web
#     (pnpm --filter @streamplace/e2e-web install-browser; needs root for
#     --with-deps, so run it inside the container).
#   * For the OAuth flow, permission to bind 127.0.0.1:443 (root, as in the
#     container). The harness serves the PDS and node over HTTPS at
#     $E2E_HTTPS_PDS_HOSTNAME and $E2E_HTTPS_STATION_HOSTNAME, public DNS
#     names for 127.0.0.1 on different registrable domains (the PDS name
#     needs a wildcard too, for handles; see pkg/cmd/e2e_https.go). Set
#     E2E_HTTPS_PDS_HOSTNAME empty to skip HTTPS; the OAuth flow then skips
#     itself.
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
if [ ! -x "$BUILDDIR/streamplace" ]; then
  echo "harness binary $BUILDDIR/streamplace missing — run 'make dev' first"
  exit 1
fi

# --- start the harness, capture its env, and always clean it up ------------
ENVFILE="$(mktemp)"
LOGFILE="$(mktemp)"
echo "starting e2e harness…"
# `streamplace e2e` forks a dev-env node, the server node and an ingest
# worker; run it in its own process group (setsid) so cleanup can take the
# whole tree down instead of orphaning the grandchildren.
#
# PIDs matching this checkout's binary before we start — never kill a scratch
# node someone else is running from the same build dir.
PREEXISTING=" $( { pgrep -f "$BUILDDIR/libstreamplace" 2>/dev/null || true; } | tr '\n' ' ') "
E2E_HTTPS_PDS_HOSTNAME="${E2E_HTTPS_PDS_HOSTNAME-localhost-pds.streamplace.network}"
E2E_HTTPS_STATION_HOSTNAME="${E2E_HTTPS_STATION_HOSTNAME-localhost-station.streamplace.team}"
HARNESS_ARGS=(e2e)
# A custom domain for the branding flow: http://localhost:<port> reaches the
# same node as SERVER_URL (http://127.0.0.1:<port>) under another hostname.
E2E_CUSTOM_DOMAINS="${E2E_CUSTOM_DOMAINS-localhost}"
for d in ${E2E_CUSTOM_DOMAINS//,/ }; do HARNESS_ARGS+=(--custom-domain "$d"); done
if [ -n "$E2E_HTTPS_PDS_HOSTNAME" ]; then
  HARNESS_ARGS+=(--https-pds-hostname "$E2E_HTTPS_PDS_HOSTNAME"
    --https-station-hostname "$E2E_HTTPS_STATION_HOSTNAME")
fi
if command -v setsid >/dev/null 2>&1; then
  setsid "$BUILDDIR/streamplace" "${HARNESS_ARGS[@]}" > "$ENVFILE" 2> "$LOGFILE" &
else
  "$BUILDDIR/streamplace" "${HARNESS_ARGS[@]}" > "$ENVFILE" 2> "$LOGFILE" &
fi
HARNESS_PID=$!
cleanup() {
  kill -- -"$HARNESS_PID" 2>/dev/null || kill "$HARNESS_PID" 2>/dev/null || true
  wait "$HARNESS_PID" 2>/dev/null || true
  # The ingest worker re-setsid's itself, escaping the group kill; sweep the
  # binaries that appeared since we started.
  for pid in $(pgrep -f "$BUILDDIR/libstreamplace" 2>/dev/null || true); do
    case "$PREEXISTING" in *" $pid "*) ;; *) kill "$pid" 2>/dev/null || true ;; esac
  done
  rm -f "$ENVFILE" "$LOGFILE"
}
trap cleanup EXIT INT TERM

for _ in $(seq 1 90); do grep -q SERVER_URL "$ENVFILE" && break; sleep 2; done
if ! grep -q SERVER_URL "$ENVFILE"; then
  echo "harness failed to start; log:"; tail -20 "$LOGFILE"; exit 1
fi
# shellcheck disable=SC1090
. "$ENVFILE"
export SERVER_URL ACCOUNT_HANDLE ACCOUNT_DID ACCOUNT_PASSWORD PDS_URL CUSTOM_DOMAINS
# only set in HTTPS mode
export SERVER_HTTPS_URL PDS_HTTPS_URL E2E_PROXY_URL E2E_TLS_SPKI
echo "harness up: SERVER_URL=$SERVER_URL ACCOUNT_HANDLE=$ACCOUNT_HANDLE${SERVER_HTTPS_URL:+ SERVER_HTTPS_URL=$SERVER_HTTPS_URL}"

# --- run the flows ---------------------------------------------------------
# No `exec` here: that would replace this shell and discard the EXIT trap,
# leaving the harness and its children running after a successful run.
cd "$REPO/js/e2e-web"
pnpm exec playwright test "$@"
