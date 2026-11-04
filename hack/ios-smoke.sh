#!/usr/bin/env bash
#
# Install the app built by `make ios-simulator` on an iPhone simulator and run
# the home Maestro flow (js/app/maestro/home.yaml) against it. The flow walks
# the app shell (Home, Videos, Settings) and needs neither Metro (the Release
# build embeds its JS) nor a backend nor a login.
#
#   hack/ios-smoke.sh
#
# Prerequisites: macOS with Xcode, jq and maestro (https://maestro.dev).
#
# Creates an isolated iPhone simulator on the newest installed iOS runtime and
# deletes only that simulator afterwards. Screenshots, Maestro reports and logs
# are retained in $E2E_ARTIFACTS/ios (default .maestro/artifacts/ios).
#
# Env overrides: APP (path to the built .app) and E2E_ARTIFACTS.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

for tool in xcrun jq plutil maestro; do
  command -v "$tool" >/dev/null || { echo "$tool not found — this script needs macOS with Xcode, jq and maestro"; exit 1; }
done

if [ -z "${APP:-}" ]; then
  set -- "$REPO"/bin/ios-simulator/Build/Products/Release-iphonesimulator/*.app
  [ $# -eq 1 ] && [ -d "$1" ] || { echo "expected one .app under bin/ios-simulator — run 'make ios-simulator'"; exit 1; }
  APP="$1"
fi
[ -d "$APP" ] || { echo "$APP is not an app bundle"; exit 1; }
# the bundle id differs between CI (tv.aquareum) and local (tv.aquareum.dev) builds
APP_ID="$(plutil -extract CFBundleIdentifier raw -o - "$APP/Info.plist")"

OUT="${E2E_ARTIFACTS:-$REPO/.maestro/artifacts}/ios"
rm -rf "$OUT"
mkdir -p "$OUT"

DEVICE="$(xcrun simctl list devices available -j | jq -r '
  [.devices | to_entries[] | select(.key | test("SimRuntime\\.iOS-")) | .key as $runtime
    | .value[] | select(.name | startswith("iPhone")) | {runtime: $runtime, deviceTypeIdentifier}]
  | max_by(.runtime | capture("iOS-(?<version>[0-9-]+)$").version | split("-") | map(tonumber))
  | select(. != null) | [.deviceTypeIdentifier, .runtime] | @tsv')"
[ -n "$DEVICE" ] || { echo "no available iPhone simulator — install an iOS simulator runtime"; exit 1; }
read -r DEVICE_TYPE RUNTIME <<<"$DEVICE"
UDID="$(xcrun simctl create "Streamplace upgrade smoke" "$DEVICE_TYPE" "$RUNTIME")"

cleanup() {
  status=$?
  trap - EXIT
  # best effort: a failed run is why these are wanted
  xcrun simctl io "$UDID" screenshot "$OUT/final.png" >/dev/null 2>&1 || true
  xcrun simctl spawn "$UDID" log show --last 15m --style compact >"$OUT/simulator.log" 2>&1 || true
  xcrun simctl shutdown "$UDID" >/dev/null 2>&1 || true
  xcrun simctl delete "$UDID" >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT

xcrun simctl boot "$UDID"
xcrun simctl bootstatus "$UDID" -b >/dev/null
echo "simulator $UDID, app $APP_ID from $(basename "$APP")"

xcrun simctl install "$UDID" "$APP"

# maestro writes takeScreenshot files to the working directory
cd "$OUT"
maestro --device "$UDID" test -e APP_ID="$APP_ID" \
  --test-output-dir "$OUT/report" --debug-output "$OUT/debug" \
  "$REPO/js/app/maestro/home.yaml"

echo "artifacts: $OUT"
