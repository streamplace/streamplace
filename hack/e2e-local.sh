#!/usr/bin/env bash
#
# Run the Maestro suite (.maestro/) against a self-contained `streamplace e2e`
# harness on a running Android emulator — the mobile counterpart to
# hack/e2e-web-local.sh.
#
#   hack/e2e-local.sh android [--upstream-station] [-- <extra maestro test args>]
#
# Prerequisites (build once, or again when app or native code changes):
#   * The harness binary: make dev
#   * An e2e APK: make android-e2e (bin/streamplace-*-android-e2e.apk).
#   * A running emulator with a Google APIs image (not Play Store, which
#     can't be rooted), and maestro (https://maestro.dev).
#
# This script only does the *run*: start the harness, set up the device, run
# the flows and tear the harness down. It reinstalls the app but does not
# rebuild it.
#
# The app talks to the harness only over HTTPS: release builds refuse
# cleartext, and the OAuth flow logs in to the harness's own PDS. So the
# harness runs in HTTPS mode (see hack/lib/e2e-harness.sh; it binds
# 127.0.0.1:443, and says how to allow that if it can't), and the emulator is
# set up, as root, to reach and trust it:
#   * a hosts file mapping the harness's names (and plc.directory) to the
#     host, bind mounted over /system/etc/hosts until the emulator reboots;
#   * its throwaway CA in the user trust store, which Chrome (the login page)
#     trusts, and which the e2e APK trusts via its network security config.
#
# Env overrides: APK, APP_ID, ANDROID_SERIAL, ANDROID_HOME,
# E2E_ARTIFACTS (default .maestro/artifacts), E2E_HARNESS_LOG, and E2E_FLOWS
# (space-separated flow files to run a subset; prerequisites are automatic).
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

PLATFORM="${1:-android}"
[ $# -gt 0 ] && shift
if [ "${1:-}" = "--upstream-station" ]; then
  export SP_E2E_UPSTREAM_STATION=true
  shift
fi
[ "${1:-}" = "--" ] && shift
case "$PLATFORM" in
  android) ;;
  ios) echo "ios is not supported by this runner yet"; exit 1 ;;
  *) echo "unknown platform: $PLATFORM (expected android)"; exit 1 ;;
esac

# --- tools, device and APK -------------------------------------------------
ANDROID_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Android/Sdk}}"
ADB="$ANDROID_HOME/platform-tools/adb"
[ -x "$ADB" ] || ADB=adb
MAESTRO="$HOME/.maestro/bin/maestro"
[ -x "$MAESTRO" ] || MAESTRO=maestro
command -v "$MAESTRO" >/dev/null || { echo "maestro not found — see https://maestro.dev"; exit 1; }

DEVICE="${ANDROID_SERIAL:-$("$ADB" devices | awk 'NR>1 && $2=="device" {print $1; exit}')}"
[ -n "$DEVICE" ] || { echo "no android emulator running"; exit 1; }
adb_() { "$ADB" -s "$DEVICE" "$@"; }

# Newest by ctime: the APK comes out of bundletool's zip with the zip's fixed
# 1980s mtime, so mtime can't tell builds apart.
APK="${APK:-$(ls -tc "$REPO"/bin/streamplace-*-android-e2e.apk "$REPO"/bin/streamplace-*-android-release.apk 2>/dev/null | head -1 || true)}"
[ -f "$APK" ] || { echo "no APK in bin/ — run 'make android-e2e'"; exit 1; }
AAPT2="$(ls -d "$ANDROID_HOME"/build-tools/*/aapt2 2>/dev/null | sort -V | tail -1 || true)"
if [ -z "${APP_ID:-}" ]; then
  [ -x "$AAPT2" ] || { echo "aapt2 not found under $ANDROID_HOME/build-tools; set APP_ID"; exit 1; }
  # the package differs between CI (tv.aquareum) and local (tv.aquareum.dev)
  # builds, so read it from the APK
  APP_ID="$("$AAPT2" dump packagename "$APK")"
fi
echo "device $DEVICE, app $APP_ID from $(basename "$APK")"
if [ -x "$AAPT2" ] && ! "$AAPT2" dump xmltree --file AndroidManifest.xml "$APK" | grep networkSecurityConfig >/dev/null; then
  echo "$(basename "$APK") does not trust the harness's CA — build it with 'make android-e2e'"
  exit 1
fi
adb_ root >/dev/null
adb_ wait-for-device
if [ "$(adb_ shell id -u | tr -d '\r')" != 0 ]; then
  echo "can't get root on $DEVICE — use a Google APIs emulator image, not a Play Store one"
  exit 1
fi

# --- harness ---------------------------------------------------------------
# the node hands OAuth logins back to the app by its bundle id
export SP_E2E_APP_BUNDLE_ID="$APP_ID"
# shellcheck source=hack/lib/e2e-harness.sh
. hack/lib/e2e-harness.sh
e2e_harness_start

# --- device ----------------------------------------------------------------
# Every build signs with its own throwaway keystore, so an earlier install
# has an incompatible signature and `install -r` would fail.
adb_ uninstall "$APP_ID" >/dev/null 2>&1 || true
adb_ install -r "$APK"
# keep the stylus handwriting tutorial from hijacking text input
adb_ shell settings put secure stylus_handwriting_enabled 0 || true
# suppress ANR/crash dialogs ("Pixel Launcher isn't responding") that
# otherwise float over the app and eat maestro's taps
adb_ shell settings put global hide_error_dialogs 1 || true

if [ "${E2E_UPSTREAM_STATION:-}" = true ]; then
  : "${UPSTREAM_STATION_URL:?upstream mode requires the station HTTPS URL}"
  : "${PDS_HTTPS_URL:?upstream mode requires the PDS HTTPS URL}"
elif [ -z "${SERVER_HTTPS_URL:-}" ]; then
  echo "the Android suite needs the harness's HTTPS mode (release builds refuse cleartext); unset E2E_HTTPS_PDS_HOSTNAME"
  exit 1
fi

# Default to the emulator's host alias. An unprivileged host using a high
# HTTPS port can instead map the fixture through adb reverse on port 443.
# E2E_ANDROID_HOST_ADDR then selects device loopback without changing issuers.
hosts="127.0.0.1 localhost
::1 ip6-localhost"
for name in "$E2E_HTTPS_STATION_HOSTNAME" "$E2E_HTTPS_PDS_HOSTNAME" "$ACCOUNT_HANDLE" plc.directory; do
  hosts="$hosts
${E2E_ANDROID_HOST_ADDR:-10.0.2.2} $name"
done
printf '%s\n' "$hosts" | adb_ shell 'while grep -q " /system/etc/hosts " /proc/mounts; do umount /system/etc/hosts; done
  cat > /data/local/tmp/sp-e2e-hosts
  chmod 644 /data/local/tmp/sp-e2e-hosts
  chcon u:object_r:system_file:s0 /data/local/tmp/sp-e2e-hosts
  mount -o bind /data/local/tmp/sp-e2e-hosts /system/etc/hosts'

# The user trust store keys certificates by OpenSSL's old subject hash.
# Each run mints a new CA, so drop the one an earlier run left behind.
ca_file="$(openssl x509 -subject_hash_old -noout -in "$E2E_TLS_CA").0"
adb_ push "$E2E_TLS_CA" /data/local/tmp/sp-e2e-ca.pem >/dev/null
adb_ shell "store=/data/misc/user/0/cacerts-added
  mkdir -p \$store
  [ -f /data/local/tmp/sp-e2e-ca-file ] && rm -f \$store/\$(cat /data/local/tmp/sp-e2e-ca-file)
  cp /data/local/tmp/sp-e2e-ca.pem \$store/$ca_file
  chown system:system \$store \$store/$ca_file
  chmod 644 \$store/$ca_file
  restorecon -R \$store
  echo $ca_file > /data/local/tmp/sp-e2e-ca-file"
# Chrome hosts the PDS's login pages in a Custom Tab. On a fresh emulator it
# opens with its first-run screens and a notification prompt, which cover
# them: skip the first (userdebug images let the debug app read its command
# line from this file) and pre-grant the second, and the app's own, which
# CI builds ask for.
adb_ shell 'echo "_ --disable-fre --no-default-browser-check --no-first-run" > /data/local/tmp/chrome-command-line'
adb_ shell am set-debug-app --persistent com.android.chrome
adb_ shell pm grant com.android.chrome android.permission.POST_NOTIFICATIONS || true
adb_ shell pm grant "$APP_ID" android.permission.POST_NOTIFICATIONS 2>/dev/null || true
# Chrome reads its command line and the trust store at startup
adb_ shell am force-stop com.android.chrome || true

# The app stamps chat records with the device's own UTC offset, in whatever
# zone the device is set to; flows that check those records need it, as
# RFC 3339 writes it (+HH:MM). Maestro runs on this host, so its scripts
# read stored records from the PDS's plain-HTTP listener.
device_offset="$(adb_ shell date +%z | tr -d '\r')"
[[ "$device_offset" =~ ^[+-][0-9]{4}$ ]] || { echo "can't read $DEVICE's UTC offset: '$device_offset'"; exit 1; }

if [ "${E2E_UPSTREAM_STATION:-}" = true ]; then
  # Keep the issuer and callback loopback names identical on host and device.
  # Merely substituting 10.0.2.2 would change the DPoP-bound request target.
  local_port="${SERVER_URL##*:}"
  adb_ reverse "tcp:$local_port" "tcp:$local_port"
  APP_SERVER_URL="$SERVER_URL"
  e2e_android_stop() {
    adb_ reverse --remove "tcp:$local_port" 2>/dev/null || true
    e2e_harness_stop
  }
  trap e2e_android_stop EXIT INT TERM
else
  APP_SERVER_URL="$SERVER_HTTPS_URL"
fi
# SP_E2E_INGEST and SP_E2E_VIDEO also pick how and what the harness streams;
# flows that need the fMP4 push (audio stems) or the AV1 stream check them.
MAESTRO_ARGS=(-e APP_ID="$APP_ID" -e SERVER_URL="$APP_SERVER_URL"
  -e SERVER_HTTP_URL="$SERVER_URL"
  -e ACCOUNT_HANDLE="$ACCOUNT_HANDLE" -e ACCOUNT_PASSWORD="$ACCOUNT_PASSWORD"
  -e ACCOUNT_DID="$ACCOUNT_DID" -e PDS_HTTP_URL="$PDS_HTTP_URL"
  -e DEVICE_UTC_OFFSET="${device_offset:0:3}:${device_offset:3}"
  -e SP_E2E_INGEST="${SP_E2E_INGEST:-whip}"
  -e SP_E2E_VIDEO="${SP_E2E_VIDEO:-h264}"
  -e E2E_UPSTREAM_STATION="${E2E_UPSTREAM_STATION:-false}")

# --- run the flows ---------------------------------------------------------
# takeScreenshot paths are relative to maestro's cwd, so run from the
# artifacts dir. No `exec`: the EXIT trap has to stop the harness.
ARTIFACTS="${E2E_ARTIFACTS:-$REPO/.maestro/artifacts}"
mkdir -p "$ARTIFACTS"
ARTIFACTS="$(cd "$ARTIFACTS" && pwd)"
# Maestro's flowsOrder accepts names, not phase globs. Generate its order
# from the selected files so new scenarios never edit a shared manifest.
if [ "${E2E_UPSTREAM_STATION:-}" = true ] && [ -z "${E2E_FLOWS:-}" ]; then
  # Upstream local nodes have no logged-out feed before successful local OAuth.
  # The generated plan includes server setup and OAuth before this scenario.
  E2E_FLOWS=".maestro/logged-in/upstream-station.yaml"
fi
bash "$REPO/hack/maestro-config.sh" ${E2E_FLOWS:-} > "$ARTIFACTS/workspace.yaml"
cd "$ARTIFACTS"
MAESTRO_CLI_NO_ANALYTICS=1 "$MAESTRO" --device "$DEVICE" test \
  --test-output-dir "$ARTIFACTS" --config "$ARTIFACTS/workspace.yaml" \
  "${MAESTRO_ARGS[@]}" "$@" "$REPO/.maestro"
