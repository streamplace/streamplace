# shellcheck shell=bash
# iOS-specific provisioning for hack/e2e-local.sh. Always use a throwaway
# Simulator so test accounts and trusted CAs never alter a developer's device.
e2e_ios_prepare() {
  [ "$(uname)" = Darwin ] || { echo "iOS e2e requires macOS and Xcode"; exit 1; }
  command -v applesimutils >/dev/null || { echo "install notification tooling: brew install wix/brew/applesimutils"; exit 1; }
  IOS_APP="${IOS_APP:-$(ls -dt "$REPO"/bin/streamplace-*-ios-e2e.app 2>/dev/null | head -1 || true)}"
  [ -d "$IOS_APP" ] || { echo "no Simulator app in bin/ — run 'make ios-e2e'"; exit 1; }
  APP_ID="$(/usr/libexec/PlistBuddy -c 'Print CFBundleIdentifier' "$IOS_APP/Info.plist")"
  # Must match the build-time EXPO_PUBLIC_PLC_URL set by make ios-e2e.
  E2E_HTTPS_PLC_HOSTNAME="${E2E_HTTPS_PLC_HOSTNAME:-plc.localhost-pds.streamplace.network}"
}

e2e_ios_stop() {
  [ -n "${E2E_IOS_DEVICE:-}" ] || return 0
  xcrun simctl shutdown "$E2E_IOS_DEVICE" >/dev/null 2>&1 || true
  xcrun simctl delete "$E2E_IOS_DEVICE" >/dev/null 2>&1 || true
  E2E_IOS_DEVICE=
  [ -z "${E2E_IOS_OFFSET_DIR:-}" ] || rm -rf "$E2E_IOS_OFFSET_DIR"
}

e2e_ios_install() {
  trap 'e2e_ios_stop; e2e_harness_stop' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  # Pick the booted iPhone's runtime/type when available, otherwise the newest
  # installed iOS runtime. IOS_SIMULATOR_UDID selects a template, not a device
  # to erase or install into.
  local template
  template="$(node - "${IOS_SIMULATOR_UDID:-}" <<'JS'
const { execFileSync } = require('node:child_process');
const devices = JSON.parse(execFileSync('xcrun', ['simctl', 'list', 'devices', 'available', '--json'])).devices;
const phones = Object.entries(devices)
  .filter(([runtime]) => runtime.includes('.iOS-'))
  .sort(([a], [b]) => b.localeCompare(a, undefined, { numeric: true }))
  .flatMap(([runtime, list]) => list.filter(d => d.isAvailable && d.deviceTypeIdentifier.includes('.iPhone-')).map(d => ({ ...d, runtime })));
const requested = process.argv[2];
const phone = requested ? phones.find(d => d.udid === requested) : phones.find(d => d.state === 'Booted') || phones[0];
if (!phone) { console.error('No available iPhone Simulator; install an iOS runtime in Xcode or check IOS_SIMULATOR_UDID'); process.exit(1); }
console.log(`${phone.deviceTypeIdentifier} ${phone.runtime}`);
JS
)"
  E2E_IOS_DEVICE="$(xcrun simctl create "Streamplace e2e $$" ${template})"
  DEVICE="$E2E_IOS_DEVICE"
  echo "Simulator $DEVICE, app $APP_ID from $(basename "$IOS_APP")"
  xcrun simctl boot "$DEVICE"
  xcrun simctl bootstatus "$DEVICE" -b
  xcrun simctl keychain "$DEVICE" add-root-cert "$E2E_TLS_CA"
  xcrun simctl install "$DEVICE" "$IOS_APP"
  applesimutils --byId "$DEVICE" --bundle "$APP_ID" --setPermissions notifications=YES
  # Query Foundation inside the selected Simulator, not the host's time zone.
  # iOS runtimes do not ship a `date` executable.
  local offset_tool
  offset_tool="$(mktemp -d)/timezone"
  E2E_IOS_OFFSET_DIR="${offset_tool%/*}"
  xcrun --sdk iphonesimulator clang -x objective-c -arch "$(uname -m)" \
    -isysroot "$(xcrun --sdk iphonesimulator --show-sdk-path)" \
    -mios-simulator-version-min=15.1 -framework Foundation -o "$offset_tool" - <<'OBJC'
#import <Foundation/Foundation.h>
int main(void) {
  @autoreleasepool {
    NSInteger seconds = NSTimeZone.localTimeZone.secondsFromGMT;
    printf("%c%02ld%02ld\n", seconds < 0 ? '-' : '+', labs(seconds) / 3600, labs(seconds) / 60 % 60);
  }
}
OBJC
  device_offset="$(xcrun simctl spawn "$DEVICE" "$offset_tool")"
  rm -rf "$E2E_IOS_OFFSET_DIR"
  E2E_IOS_OFFSET_DIR=
}
