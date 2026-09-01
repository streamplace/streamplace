#!/usr/bin/env bash
#
# Run the full Maestro e2e suite locally against a self-contained
# `streamplace e2e` harness.
#
#   hack/e2e-local.sh [android|ios]     # platform; default: android on
#                                       # Linux, ios on macOS
#
# Prerequisites (build these once, or when app/native code changes):
#   * The dev streamplace binary:  make dev
#   * android: a release APK:       make android-release   (+ a running emulator)
#   * ios:     a simulator .app built with CI=true (see js/app build below)
#
# This script only does the *run*: start the harness, set up the device,
# run the flows, and tear the harness down. It re-installs the app but does
# not rebuild it.
#
# Env overrides: APP_ID, SIM_UDID (ios).
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

PLATFORM="${1:-}"
if [ -z "$PLATFORM" ]; then
  [ "$(uname)" = "Darwin" ] && PLATFORM=ios || PLATFORM=android
fi

# --- locate the dev harness binary (build-<os>-<arch>/streamplace) ---------
case "$(uname)-$(uname -m)" in
  Linux-x86_64)  BUILDDIR=build-linux-amd64 ;;
  Darwin-arm64)  BUILDDIR=build-darwin-arm64 ;;
  Darwin-x86_64) BUILDDIR=build-darwin-amd64 ;;
  *) echo "unsupported host $(uname)-$(uname -m)"; exit 1 ;;
esac
if [ ! -x "$BUILDDIR/streamplace" ]; then
  echo "harness binary $BUILDDIR/streamplace missing — run 'make dev' first"
  exit 1
fi

MAESTRO="$HOME/.maestro/bin/maestro"
[ -x "$MAESTRO" ] || MAESTRO=maestro

# --- start the harness, capture its env, and always clean it up ------------
ENVFILE="$(mktemp)"
LOGFILE="$(mktemp)"
echo "starting e2e harness…"
"$BUILDDIR/streamplace" e2e > "$ENVFILE" 2> "$LOGFILE" &
HARNESS_PID=$!
cleanup() {
  kill "$HARNESS_PID" 2>/dev/null || true
  # the harness forks a node subprocess and a WHIP streamer; reap them too
  pkill -P "$HARNESS_PID" 2>/dev/null || true
  rm -f "$ENVFILE" "$LOGFILE"
}
trap cleanup EXIT

for _ in $(seq 1 90); do [ -s "$ENVFILE" ] && break; sleep 2; done
if ! grep -q SERVER_URL "$ENVFILE"; then
  echo "harness failed to start; log:"; tail -20 "$LOGFILE"; exit 1
fi
# shellcheck disable=SC1090
. "$ENVFILE"
echo "harness up: SERVER_URL=$SERVER_URL ACCOUNT_HANDLE=$ACCOUNT_HANDLE"

run_maestro() {
  local device="$1" appid="$2" serverurl="$3"
  echo "running .maestro/ against $appid on $device …"
  MAESTRO_CLI_NO_ANALYTICS=1 "$MAESTRO" --device "$device" test \
    -e APP_ID="$appid" \
    -e SERVER_URL="$serverurl" \
    -e ACCOUNT_HANDLE="$ACCOUNT_HANDLE" \
    .maestro/
}

if [ "$PLATFORM" = "android" ]; then
  ADB="$HOME/Android/Sdk/platform-tools/adb"; [ -x "$ADB" ] || ADB=adb
  DEVICE="$("$ADB" devices | awk '/emulator|device$/{print $1; exit}')"
  [ -n "$DEVICE" ] || { echo "no android emulator/device running"; exit 1; }
  APK="$(ls -t "$REPO"/bin/streamplace-*-android-release.apk 2>/dev/null | head -1)"
  [ -n "$APK" ] || { echo "no APK in bin/ — run 'make android-release'"; exit 1; }
  APP_ID="${APP_ID:-tv.aquareum.dev}"
  # the emulator reaches the host loopback via 10.0.2.2
  SERVER_URL="$(echo "$SERVER_URL" | sed 's/127\.0\.0\.1/10.0.2.2/')"
  "$ADB" -s "$DEVICE" uninstall "$APP_ID" >/dev/null 2>&1 || true
  "$ADB" -s "$DEVICE" install -r "$APK"
  "$ADB" -s "$DEVICE" shell settings put secure stylus_handwriting_enabled 0 || true
  "$ADB" -s "$DEVICE" shell settings put global hide_error_dialogs 1 || true
  run_maestro "$DEVICE" "$APP_ID" "$SERVER_URL"

elif [ "$PLATFORM" = "ios" ]; then
  APP="$REPO/js/app/ios/build/Build/Products/Release-iphonesimulator/Streamplace.app"
  [ -d "$APP" ] || { echo "no simulator .app — build it (CI=true pnpm run build; pod install; xcodebuild … -sdk iphonesimulator)"; exit 1; }
  APP_ID="${APP_ID:-$(/usr/libexec/PlistBuddy -c 'Print CFBundleIdentifier' "$APP/Info.plist")}"
  # Use a dedicated "e2e-iphone" simulator, creating it once if needed. Then
  # ALWAYS erase it for a clean state before the run — a freshly *created*
  # sim doesn't reliably accept the applesimutils notification pre-grant,
  # but a freshly *erased* one does (and clean state matches CI).
  SIM="${SIM_UDID:-$(xcrun simctl list devices | grep -A0 'e2e-iphone (' | grep -Eo '[0-9A-F-]{36}' | head -1 || true)}"
  if [ -z "$SIM" ]; then
    DEVTYPE="$(xcrun simctl list devicetypes | grep -oE 'iPhone [0-9]+' | sort -k2 -n | tail -1)"
    RUNTIME="$(xcrun simctl list runtimes | grep -Eo 'com.apple.CoreSimulator.SimRuntime.iOS-[0-9-]+' | tail -1)"
    SIM="$(xcrun simctl create e2e-iphone "$DEVTYPE" "$RUNTIME")"
  fi
  xcrun simctl shutdown "$SIM" 2>/dev/null || true
  xcrun simctl erase "$SIM"
  xcrun simctl boot "$SIM"
  xcrun simctl bootstatus "$SIM" -b
  xcrun simctl install "$SIM" "$APP"
  # pre-grant notifications so the first-launch permission dialog (which
  # dims the screen and blocks taps) never appears
  applesimutils --byId "$SIM" --bundle "$APP_ID" --setPermissions notifications=YES
  # iOS simulator shares the host loopback — no 10.0.2.2 rewrite
  run_maestro "$SIM" "$APP_ID" "$SERVER_URL"

else
  echo "unknown platform: $PLATFORM (expected android or ios)"; exit 1
fi
