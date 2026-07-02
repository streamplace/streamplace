#!/usr/bin/env bash
# Runs the Maestro e2e suite against an already-booted Android emulator.
# This must be a single script (not inline workflow lines) because
# reactivecircus/android-emulator-runner executes each line of `script:`
# in a separate shell, so environment variables don't survive between lines.
set -euo pipefail

. /tmp/e2e-env.sh
if [ -z "${SERVER_URL:-}" ]; then
  echo "SERVER_URL missing from /tmp/e2e-env.sh"
  exit 1
fi
# the emulator reaches the host's loopback via 10.0.2.2
ANDROID_SERVER_URL="$(echo "$SERVER_URL" | sed 's/127\.0\.0\.1/10.0.2.2/')"

adb install -r ./apk/*.apk
# keep the stylus handwriting tutorial from hijacking text input
adb shell settings put secure stylus_handwriting_enabled 0 || true
# suppress ANR/crash dialogs ("Pixel Launcher isn't responding") that
# otherwise float over the app and eat maestro's taps
adb shell settings put global hide_error_dialogs 1 || true

maestro test \
  -e APP_ID=tv.aquareum \
  -e SERVER_URL="$ANDROID_SERVER_URL" \
  -e ACCOUNT_HANDLE="$ACCOUNT_HANDLE" \
  .maestro/
