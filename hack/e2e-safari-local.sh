#!/usr/bin/env bash
# Run the chat smoke test in Safari on a disposable iPhone simulator.
# See js/e2e-web/README.md for the macOS HTTPS and SafariDriver setup.
set -euo pipefail

[ "$(uname)" = Darwin ] || { echo 'Safari e2e requires macOS and Xcode' >&2; exit 1; }
cd "$(dirname "$0")/.."
export E2E_ARTIFACTS="${E2E_ARTIFACTS:-$PWD/js/e2e-web/test-results/safari}"
export E2E_HARNESS_LOG="$E2E_ARTIFACTS/harness.log"
export SAFARIDRIVER_PORT="${SAFARIDRIVER_PORT:-4444}"
mkdir -p "$E2E_ARTIFACTS"

# shellcheck source=hack/lib/e2e-harness.sh
. hack/lib/e2e-harness.sh
# shellcheck source=hack/lib/e2e-ios.sh
. hack/lib/e2e-ios.sh
e2e_harness_start

cleanup() {
  local status=$?
  if [ -n "${driver_pid:-}" ]; then
    kill "$driver_pid" 2>/dev/null || true
    wait "$driver_pid" 2>/dev/null || true
  fi
  if [ "$status" -ne 0 ] && [ -n "${E2E_IOS_DEVICE:-}" ]; then
    xcrun simctl io "$E2E_IOS_DEVICE" screenshot "$E2E_ARTIFACTS/simulator.png" || true
  fi
  e2e_ios_stop
  e2e_harness_stop
}
trap cleanup EXIT

: "${E2E_TLS_CA:?Safari chat requires the harness HTTPS hostnames}"
e2e_ios_boot
export E2E_IOS_DEVICE
xcrun simctl keychain "$E2E_IOS_DEVICE" add-root-cert "$E2E_TLS_CA"
/usr/bin/safaridriver --port "$SAFARIDRIVER_PORT" >"$E2E_ARTIFACTS/safaridriver.log" 2>&1 &
driver_pid=$!
node js/e2e-web/safari/chat.mts
