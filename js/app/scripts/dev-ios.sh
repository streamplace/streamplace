#!/bin/bash
# Launches the Devplace dev client on the iOS simulator, pointed at the local
# Metro dev server. This is the fast JS loop: it never rebuilds native code.
#
# Prereqs:
#   - Metro is running on :38081 (pnpm app start:fast)
#   - The dev client is installed on the simulator (pnpm app ios, once)
#
# Same env vars as app.config.ts: SP_METRO_PORT, SP_APP_SCHEME, SP_BUNDLE_OVERRIDE

set -euo pipefail

PORT="${SP_METRO_PORT:-38081}"
BUNDLE="${SP_BUNDLE_OVERRIDE:-tv.aquareum.dev}"
SCHEME="${SP_APP_SCHEME:-$BUNDLE}"
METRO_URL="http://127.0.0.1:${PORT}"
DEV_CLIENT_URL="${SCHEME}://expo-development-client/?url=$(
  node -e 'process.stdout.write(encodeURIComponent(process.argv[1]))' "$METRO_URL"
)"

if ! curl -sf --max-time 2 "$METRO_URL" >/dev/null 2>&1; then
  echo "Metro isn't running on :${PORT}. Start it with: pnpm app start:fast" >&2
  exit 1
fi

# Boot a simulator if none is booted.
if ! xcrun simctl list devices booted | grep -qi "booted"; then
  SIM="$(xcrun simctl list devices available |
    grep -i "iphone" |
    grep -vi unavailable |
    head -1 |
    sed -E 's/.*\(([0-9A-F-]{36})\).*/\1/')"
  if [ -z "$SIM" ]; then
    echo "No iPhone simulator found" >&2
    exit 1
  fi
  echo "Booting simulator ${SIM}"
  xcrun simctl boot "$SIM" >/dev/null
  xcrun simctl bootstatus "$SIM" -b >/dev/null
  open -a Simulator
fi

if ! xcrun simctl get_app_container booted "$BUNDLE" app >/dev/null 2>&1; then
  echo "Dev client (${BUNDLE}) isn't installed on the simulator.
Run once: pnpm app ios" >&2
  exit 1
fi

xcrun simctl openurl booted "$DEV_CLIENT_URL"
echo "Opened ${SCHEME} dev client -> ${METRO_URL}"
