# shellcheck shell=bash
# Start a self-contained `streamplace e2e` harness for a test runner, and
# always tear it down. Sourced by hack/e2e-web-local.sh and hack/e2e-local.sh.
#
#   . hack/lib/e2e-harness.sh
#   e2e_harness_start
#
# The caller's cwd must be the repo root. On return the harness is up and its
# variables are exported: SERVER_URL, ACCOUNT_HANDLE, ACCOUNT_DID,
# ACCOUNT_PASSWORD, and in HTTPS mode also SERVER_HTTPS_URL, PDS_HTTPS_URL,
# E2E_PROXY_URL, E2E_TLS_SPKI and E2E_TLS_CA. An EXIT trap stops it; a caller
# with its own EXIT work calls e2e_harness_stop from its trap instead.
#
# HTTPS mode (for the OAuth flows) is on by default and needs permission to
# bind 127.0.0.1:443 (root, as in the build container). The harness serves the
# PDS and node over HTTPS at $E2E_HTTPS_PDS_HOSTNAME and
# $E2E_HTTPS_STATION_HOSTNAME, public DNS names for 127.0.0.1 on different
# registrable domains (the PDS name needs a wildcard too, for handles; see
# pkg/cmd/e2e_https.go). Set E2E_HTTPS_PDS_HOSTNAME empty to skip HTTPS; the
# OAuth flows then skip themselves. A machine that redirects loopback 443
# elsewhere (an iptables REDIRECT rule) sets E2E_HTTPS_PORT to the target port.
#
# E2E_HARNESS_LOG, if set, is where the harness log goes; it is kept after the
# run (CI uploads it). Otherwise the log is a temp file, shown on a failed
# start and deleted afterwards.

case "$(uname)-$(uname -m)" in
  Linux-x86_64)  BUILDDIR=build-linux-amd64 ;;
  Linux-aarch64) BUILDDIR=build-linux-arm64 ;;
  Darwin-arm64)  BUILDDIR=build-darwin-arm64 ;;
  Darwin-x86_64) BUILDDIR=build-darwin-amd64 ;;
  *) echo "unsupported host $(uname)-$(uname -m)"; exit 1 ;;
esac

e2e_harness_start() {
  if [ ! -x "$BUILDDIR/streamplace" ]; then
    echo "harness binary $BUILDDIR/streamplace missing — run 'make dev' first"
    exit 1
  fi

  E2E_ENVFILE="$(mktemp)"
  if [ -n "${E2E_HARNESS_LOG:-}" ]; then
    E2E_LOGFILE="$E2E_HARNESS_LOG"
    E2E_KEEP_LOG=1
    mkdir -p "$(dirname "$E2E_LOGFILE")"
  else
    E2E_LOGFILE="$(mktemp)"
    E2E_KEEP_LOG=
  fi
  echo "starting e2e harness…"
  # `streamplace e2e` forks a dev-env node, the server node and an ingest
  # worker; run it in its own process group (setsid) so cleanup can take the
  # whole tree down instead of orphaning the grandchildren.
  #
  # PIDs matching this checkout's binary before we start — never kill a
  # scratch node someone else is running from the same build dir.
  E2E_PREEXISTING=" $( { pgrep -f "$BUILDDIR/libstreamplace" 2>/dev/null || true; } | tr '\n' ' ') "
  E2E_HTTPS_PDS_HOSTNAME="${E2E_HTTPS_PDS_HOSTNAME-localhost-pds.streamplace.network}"
  E2E_HTTPS_STATION_HOSTNAME="${E2E_HTTPS_STATION_HOSTNAME-localhost-station.streamplace.team}"
  local args=(e2e)
  if [ -n "$E2E_HTTPS_PDS_HOSTNAME" ]; then
    args+=(--https-pds-hostname "$E2E_HTTPS_PDS_HOSTNAME"
      --https-station-hostname "$E2E_HTTPS_STATION_HOSTNAME"
      --https-port "${E2E_HTTPS_PORT:-443}")
  fi
  if command -v setsid >/dev/null 2>&1; then
    setsid "$BUILDDIR/streamplace" "${args[@]}" > "$E2E_ENVFILE" 2> "$E2E_LOGFILE" &
  else
    "$BUILDDIR/streamplace" "${args[@]}" > "$E2E_ENVFILE" 2> "$E2E_LOGFILE" &
  fi
  E2E_HARNESS_PID=$!
  trap e2e_harness_stop EXIT INT TERM

  for _ in $(seq 1 90); do grep -q SERVER_URL "$E2E_ENVFILE" && break; sleep 2; done
  if ! grep -q SERVER_URL "$E2E_ENVFILE"; then
    echo "harness failed to start; log:"; tail -20 "$E2E_LOGFILE"; exit 1
  fi
  # shellcheck disable=SC1090
  . "$E2E_ENVFILE"
  export SERVER_URL ACCOUNT_HANDLE ACCOUNT_DID ACCOUNT_PASSWORD
  # only set in HTTPS mode
  export SERVER_HTTPS_URL PDS_HTTPS_URL E2E_PROXY_URL E2E_TLS_SPKI E2E_TLS_CA
  echo "harness up: SERVER_URL=$SERVER_URL ACCOUNT_HANDLE=$ACCOUNT_HANDLE${SERVER_HTTPS_URL:+ SERVER_HTTPS_URL=$SERVER_HTTPS_URL}"
}

e2e_harness_stop() {
  [ -n "${E2E_HARNESS_PID:-}" ] || return 0
  kill -- -"$E2E_HARNESS_PID" 2>/dev/null || kill "$E2E_HARNESS_PID" 2>/dev/null || true
  wait "$E2E_HARNESS_PID" 2>/dev/null || true
  # The ingest worker re-setsid's itself, escaping the group kill; sweep the
  # binaries that appeared since we started.
  local pid
  for pid in $(pgrep -f "$BUILDDIR/libstreamplace" 2>/dev/null || true); do
    case "$E2E_PREEXISTING" in *" $pid "*) ;; *) kill "$pid" 2>/dev/null || true ;; esac
  done
  rm -f "$E2E_ENVFILE"
  [ -n "$E2E_KEEP_LOG" ] || rm -f "$E2E_LOGFILE"
  E2E_HARNESS_PID=
}
