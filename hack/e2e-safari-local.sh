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
  if [ "$status" -ne 0 ] && [ -n "${driver_pid:-}" ]; then
    local log
    for log in "$HOME/Library/Logs/com.apple.WebDriver/"*"$driver_pid"*; do
      [ ! -f "$log" ] || cp "$log" "$E2E_ARTIFACTS/" || true
    done
  fi
  if [ "$status" -ne 0 ] && [ -n "${E2E_IOS_DEVICE:-}" ]; then
    xcrun simctl io "$E2E_IOS_DEVICE" screenshot "$E2E_ARTIFACTS/simulator.png" || true
    xcrun simctl spawn "$E2E_IOS_DEVICE" log show --last 5m --style compact \
      --predicate 'process == "MobileSafari" OR process == "webinspectord"' \
      >"$E2E_ARTIFACTS/simulator-safari.log" 2>&1 || true
  fi
  e2e_ios_stop
  e2e_harness_stop
}
trap cleanup EXIT

: "${E2E_TLS_CA:?Safari chat requires the harness HTTPS hostnames}"
# Start the driver while CoreSimulator boots so both are ready for the session.
/usr/bin/safaridriver -p "$SAFARIDRIVER_PORT" --diagnose >"$E2E_ARTIFACTS/safaridriver.log" 2>&1 &
driver_pid=$!
e2e_ios_boot
export E2E_IOS_DEVICE
xcrun simctl keychain "$E2E_IOS_DEVICE" add-root-cert "$E2E_TLS_CA"
# Launch Safari explicitly so a cold simulator does not spend the driver's
# session handshake deadline starting its browser.
xcrun simctl launch "$E2E_IOS_DEVICE" com.apple.mobilesafari
if ! kill -0 "$driver_pid" 2>/dev/null; then
  echo 'SafariDriver exited before the test started' >&2
  tail -20 "$E2E_ARTIFACTS/safaridriver.log" >&2
  exit 1
fi
node js/e2e-web/safari/chat.mts
