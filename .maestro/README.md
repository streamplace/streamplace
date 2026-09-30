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

## Switching accounts

The harness has a second account, `DEPRECATED_HOST_ACCOUNT_*`, which streams
RTMP through a hostname the node treats as deprecated
(`--deprecated-ingest-hosts`). `logged-in/deprecated-ingest-host.yaml` signs
out, signs in as that account to check its live dashboard warns it to change
server, then signs back in as the harness account, leaving the phase as it
found it. It uses `setup/sign-out.yaml` and `setup/sign-in.yaml` (with
`HANDLE`/`PASSWORD` env), which tolerate the PDS skipping its password or
consent page for a session the Custom Tab already holds.

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
