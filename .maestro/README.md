# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app, run against a self-contained
`streamplace e2e` harness (a local PDS and PLC, a Streamplace node and a
looping test stream). `hack/e2e-local.sh android` runs them on an emulator,
and so does the `android-e2e` job in `.github/workflows/build.yaml`. iOS is not
wired up yet. The web suite (`js/e2e-web`, `hack/e2e-web-local.sh`) mirrors
these flows.

## Organization and prerequisites

Use feature/behavior names, not sequence numbers. Add a lowercase kebab-case
flow file to `logged-out/` or `logged-in/`, depending on its required session
state. For example, a new signed-in chat scenario is
`logged-in/chat-message-expiry.yaml`; its web counterpart is
`js/e2e-web/flows/chat-message-expiry.spec.ts`. Filenames must be unique across
the mobile suite; do not override the flow's `name` property.

The runner generates `.maestro/artifacts/workspace.yaml` using
`hack/maestro-config.sh`. It discovers scenario files and orders **phases**:

1. `setup/server-setup.yaml` points the app at the harness.
2. Every `logged-out/*.yaml` scenario runs.
3. `setup/oauth-login.yaml` signs in and verifies persisted chat.
4. Every `logged-in/*.yaml` scenario runs.

Maestro only accepts individual names in `executionOrder.flowsOrder`, not
phase globs, so that list is generated at runtime, never committed. Adding a
scenario does not require editing a manifest, this README, or another test.
Alphabetical order within each phase is only for reproducible reports, not a
dependency contract. Every scenario must launch/navigate to its own starting
screen and create its own test data; it must not depend on another scenario's
messages or navigation. A flow must leave its phase's session state intact.
Reusable prerequisite flows belong in `setup/`, outside scenario discovery.

Use `hack/e2e-local.sh android` rather than running the directory without its
generated config. For a subset, prerequisites are included automatically:

```bash
E2E_FLOWS=".maestro/logged-in/chat-profile.yaml" hack/e2e-local.sh android
# Inspect the selected plan without a device:
bash hack/maestro-config.sh .maestro/logged-in/chat-profile.yaml
```

The organization and generated config are platform-neutral; the current runner
only provisions Android. No native Go Live scenario exists: native builds hide
those controls, and `logged-out/tabs.yaml` checks they stay hidden.

## Chat timestamp coverage

`logged-in/chat-timezone.yaml` sends a unique message, verifies it survives a
relaunch, then reads the actual `place.stream.chat.message` record from the PDS.
It requires a numeric RFC 3339 offset matching the device, and checks that the
timestamp represents the send instant rather than UTC mislabeled as local time.
The shared flow works on iOS and Android; the Android runner supplies
`ACCOUNT_DID`, `PDS_HTTP_URL` (the harness's loopback PDS listener), and
`DEVICE_UTC_OFFSET` (`+HH:MM`, read with `adb shell date +%z`).
Maestro's JavaScript runs on the host, so its own local time zone is not a device
offset oracle. An iOS invocation must supply the same variables using the
simulator's offset (normally the Mac host's).

The Web counterpart, `js/e2e-web/flows/chat-timezone.spec.ts`, exercises both the
Expo and Vite send paths in Los Angeles, Kathmandu, and UTC.

## Profile coverage

`logged-in/chat-profile.yaml` taps the signed-in chat author's name, checks that the profile
sheet contains that account's handle and the View Profile action, dismisses it
by swiping its handle down, and opens it again. It then sends another message without
restarting and verifies that message survives a relaunch from server history.
The dismissal uses the same sheet handle gesture on iOS and Android, not Android
Back. Android flattens nested username text, so the flow taps near the start of
the matching message row. The logged-in Playwright flow (`oauth-login.spec.ts`) covers the matching
open/dismiss/reopen/chat sequence in the Expo web app served by the default harness.

Native dropdowns render through the default `@rn-primitives` portal host. That
host belongs below the app stores, i18n, branded theme and font providers so
profile content retains their context. The sheet also has its own error boundary:
unexpected content errors show a menu-local fallback with Try again and Dismiss,
rather than replacing the app. The profile flow requires actual profile content
and rejects that fallback.

`pnpm --filter @streamplace/components test` covers failures in both child
components and menu render callbacks, repeated and successful retries,
dismissal/reopening, and preservation of the surrounding screen's unsent draft.
These focused tests use native presentation adapters; the Maestro flow exercises
the actual native sheet and portal. The component tests also run in
`pnpm run check`.

## HTTPS, and logging in

The app reaches the harness over HTTPS only: release builds refuse cleartext,
and `setup/oauth-login.yaml` signs in to the harness's own PDS through the real atproto
OAuth flow (the node's OAuth proxy, then the PDS's sign-in and consent pages
in a Chrome Custom Tab). So the harness runs in its HTTPS mode, serving public
names for 127.0.0.1 with a throwaway CA (see `pkg/cmd/e2e_https.go`), and the
runner sets the emulator up, as root, to reach and trust it:

- a hosts file maps the harness's names, and `plc.directory` (the app
  resolves `did:plc` there itself), to the host at 10.0.2.2;
- the CA goes in the user trust store. Chrome trusts that store; the app only
  does when built with `make android-e2e` (`SP_E2E_BUILD` in
  `js/app/app.config.ts`, which also turns off OTA updates so the run tests
  the JS it was built with). Never ship that build.

## Run it locally

```bash
make dev           # harness binary
make android-e2e   # bin/streamplace-*-android-e2e.apk (needs JDK 17 + ANDROID_HOME)
# boot a Google APIs emulator (Play Store images can't be rooted), e.g.
sdkmanager "system-images;android-34;google_apis;x86_64"
avdmanager create avd -n e2e-api34 -k "system-images;android-34;google_apis;x86_64" -d pixel_6
emulator -avd e2e-api34 -no-window -no-audio -no-snapshot-save &
hack/e2e-local.sh android
```

The runner starts the harness, installs the APK, prepares the emulator, runs
the flows and tears everything down. Screenshots, maestro's logs and the
report land in `.maestro/artifacts/`.

The runner runs on the host, not in the build container: the emulator has to
reach the harness.

### Port 443

The harness binds 127.0.0.1:443, which an unprivileged host process can't do
by default; if it fails, it says how to allow that until the next reboot. To
set a development machine up once instead, redirect loopback 443 to a high
port with a systemd unit, and point the harness at that port:

```ini
# /etc/systemd/system/loopback-443-redirect.service
[Unit]
Description=Redirect loopback port 443 to 38444 (Streamplace e2e harness, E2E_HTTPS_PORT=38444)
After=network-pre.target

[Service]
Type=oneshot
RemainAfterExit=yes
# -C first so starting it again never stacks a duplicate rule
ExecStart=/bin/sh -c '/usr/sbin/iptables -t nat -C OUTPUT -o lo -p tcp --dport 443 -j REDIRECT --to-ports 38444 2>/dev/null || /usr/sbin/iptables -t nat -A OUTPUT -o lo -p tcp --dport 443 -j REDIRECT --to-ports 38444'
ExecStop=/usr/sbin/iptables -t nat -D OUTPUT -o lo -p tcp --dport 443 -j REDIRECT --to-ports 38444

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now loopback-443-redirect.service
E2E_HTTPS_PORT=38444 hack/e2e-local.sh android
```

The rule covers the emulator too: its connections to the host (10.0.2.2)
leave from the emulator process on the host's loopback. It only affects
connections to loopback 443, so it can get in the way of anything else that
serves HTTPS on 127.0.0.1:443 on that machine. Only set `E2E_HTTPS_PORT` when
the redirect is in place: the app and PDS still use `https://<host>` URLs
with no port.

## Upstream-station native flow

The default single-node plan excludes flows tagged `upstream-station`. Opt in
with the real atmoq executable built from the pinned, certificate-watcher-fixed
commit in [the Web harness prerequisites](../js/e2e-web/README.md#upstream-station-topology):

```bash
make android-e2e
SP_E2E_ATMOQ_BIN=/path/to/atmoq \
  hack/e2e-local.sh android --upstream-station
# Explicit subset, including server setup and real OAuth prerequisites:
SP_E2E_ATMOQ_BIN=/path/to/atmoq \
  E2E_FLOWS=.maestro/logged-in/upstream-station.yaml \
  hack/e2e-local.sh android --upstream-station
```

The runner selects the local HTTP `SERVER_URL`, not the HTTPS upstream station.
It creates an owned `adb reverse tcp:<local-port> tcp:<local-port>` mapping and
removes it at teardown. Using `10.0.2.2` as the node URL would change the
DPoP-bound issuer/callback, so the app keeps `http://127.0.0.1:<local-port>`.
The E2E-only network security plugin allows cleartext solely for `127.0.0.1`
and `localhost`; ordinary releases and all other E2E destinations remain
HTTPS-only. The station/PDS still use genuine TLS and the installed throwaway
CA. The harness prints local/station/MoQ URLs, and uses actual per-DID QUIC
subscriptions rather than a mock firehose.

`setup/oauth-login.yaml` completes the local → HTTPS station → PDS consent
chain. Upstream mode defers fixture go-live until this login, so its first-card
wait allows 60 seconds for initial segments and the existing 30-second feed-cache
TTL; standalone setup has already established go-live and retains its 30-second
wait. `logged-in/upstream-station.yaml` performs an authenticated chat write,
kills/relaunches the app, checks indexed history rather than an optimistic
bubble, and writes again with the restored OAuth session. It is platform-neutral
and uses the same semantic selectors on Android and iOS. Browser coverage adds
transport audits, renewal, the actual Go-node restart and final-user revocation
checks; native coverage is login/write/app-relaunch, not a claim those lower-level
assertions were run on a device.

### iOS Simulator manual recipe (macOS required)

The Linux runner cannot provision an iOS Simulator. On a macOS development
machine, build/install the current app on a booted simulator, pre-grant its
notification permission, and start the same harness from the checkout root:

```bash
# Keep this shell alive; its EXIT trap owns the harness and the ephemeral CA.
export SP_E2E_UPSTREAM_STATION=true
export SP_E2E_ATMOQ_BIN=/path/to/atmoq
export SP_E2E_APP_BUNDLE_ID=<installed-app-bundle-id>
. hack/lib/e2e-harness.sh
e2e_harness_start
xcrun simctl keychain booted add-root-cert "$E2E_TLS_CA"
SP_E2E_UPSTREAM_STATION=true bash hack/maestro-config.sh \
  .maestro/logged-in/upstream-station.yaml > /tmp/streamplace-ios-upstream.yaml
maestro --device <booted-simulator-udid> test \
  --config /tmp/streamplace-ios-upstream.yaml \
  -e APP_ID="$SP_E2E_APP_BUNDLE_ID" -e SERVER_URL="$SERVER_URL" \
  -e ACCOUNT_HANDLE="$ACCOUNT_HANDLE" -e ACCOUNT_PASSWORD="$ACCOUNT_PASSWORD" \
  -e E2E_UPSTREAM_STATION=true .maestro
```

Simulator networking must also resolve the harness's `plc.directory` to this
run's local PLC TLS front end, not the public directory: arrange a temporary
local DNS/hosts override or a simulator CONNECT proxy using `E2E_PROXY_URL`,
and remove only that override afterward. The station/PDS/account DNS names
already need to resolve to host loopback as described above. Trust the CA in
the simulator, not the macOS system keychain. Confirm the installed build's
generated `Info.plist` permits local-network transport with
`NSAllowsLocalNetworking=true`; do not enable blanket arbitrary HTTP loads.
Remove the temporary workspace file and the simulator's test CA after the run.
These are execution prerequisites, not
evidence that an iOS run has occurred.

## Notes / gotchas

- **Other adb devices:** maestro lists no Android devices at all ("Device
  emulator-5554 was requested, but it is not connected") while any device
  adb knows about is `unauthorized`, such as a phone plugged in over USB that
  hasn't allowed this computer. Authorize or unplug it, or restart the adb
  server so it leaves USB devices alone:
  `adb kill-server && adb --one-device emulator-5554 start-server`.
- **Android dialogs:** the runner turns off ANR/crash dialogs ("Pixel Launcher
  isn't responding") and the stylus handwriting tutorial, both of which eat
  taps.
- **Signing:** every build signs with its own throwaway keystore, so the
  runner uninstalls any earlier copy before installing.
- **iOS notification dialog:** the first-launch "would like to send you
  notifications" prompt dims the whole screen and blocks taps. It only shows
  on a fresh install; pre-grant the permission with `applesimutils` and avoid
  `clearState` (which reinstalls and re-triggers it).
- **iOS accessibility collapse:** iOS merges tappable containers (the settings
  toggle, the stream cards, the login modal) into a single accessibility
  element, so those are matched by `testID` or substring regex, not exact text.
