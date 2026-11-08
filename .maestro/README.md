# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app, run against a self-contained
`streamplace e2e` harness (a local PDS and PLC, a Streamplace node and a
looping test stream). `hack/e2e-local.sh android` runs them on an emulator;
`hack/e2e-local.sh ios` runs the same suite in a disposable iOS Simulator.
Android also runs in the `android-e2e` CI job. The `ios` CI job builds
`make ios-simulator` and runs the basic `js/app/maestro/home.yaml` smoke flow
through `hack/ios-smoke.sh`; the full iOS suite runs locally.
The web suite (`js/e2e-web`, `hack/e2e-web-local.sh`) mirrors these flows.

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

Use `hack/e2e-local.sh android` or `hack/e2e-local.sh ios`, rather than running
the directory without its generated config. Subsets include prerequisites:

```bash
E2E_FLOWS=".maestro/logged-in/chat-profile.yaml" hack/e2e-local.sh android
# Inspect the selected plan without a device:
bash hack/maestro-config.sh .maestro/logged-in/chat-profile.yaml
```

The organization and generated config work on both platforms, including macOS's
stock Bash. No native Go Live scenario exists: native builds hide those controls,
and `logged-out/tabs.yaml` checks they stay hidden.

## Live avatar coverage

`logged-out/user-search.yaml` captures a cropped live avatar on iOS and Android.
Android compares it with `screenshots/android-live-avatar.png` (420 dpi, dark
theme) to catch square rings. The web search test checks ring/corner pixels;
the avatar unit test covers scaled and custom sizes.

## Chat submission coverage

`logged-in/chat-submit.yaml` sends with keyboard Return, then the chat Send
button, then Return again without erasing or tapping the input between messages.
Exact message rows and server history catch a sent draft being retained and
prepended to the next message. The flow is shared by iOS and Android;
`oauth-login.spec.ts` covers keyboard/button sending and empty fields on Web.

The component regression exercises stale React drafts, final native text,
single submission, the 300-grapheme limit, preservation of the next draft,
composer identity, and text-responder continuity.
On iOS both Send controls transfer focus to a permanently mounted, nonvisible
companion input and back to the composer. That ends editing to commit pending
autocorrection without explicitly dismissing the keyboard. Both inputs use the
same textarea wrapper and keyboard traits, including bottom-sheet integration.
Submission still waits for the composer's final `onEndEditing` text before
clearing. The companion is excluded from touch and accessibility navigation.
Ordinary blur does not send.

Maestro `inputText` does not drive UIKit's autocorrection UI. On a real iOS
keyboard with Auto-Correction enabled, type a sentence ending in a misspelling
with a visible correction suggestion, without adding a trailing space. Test the
keyboard Send key and the app's Send button separately. Each must send exactly
one corrected message, leave an empty input, and allow a fresh next message
without the keyboard dropping down and reopening.
Moving focus away without pressing either Send control must not send anything.

## Chat timestamp coverage

`logged-in/chat-timezone.yaml` sends a unique message, verifies it survives a
relaunch, then reads the actual `place.stream.chat.message` record from the PDS.
It requires a numeric RFC 3339 offset matching the device, and checks that the
timestamp represents the send instant rather than UTC mislabeled as local time.
Both platform runners supply `ACCOUNT_DID`, `PDS_HTTP_URL` (the harness's loopback
PDS listener), and `DEVICE_UTC_OFFSET` (`+HH:MM`). Android reads the offset with
`adb shell date +%z`; iOS runs a small Foundation executable inside the selected
Simulator. Maestro's JavaScript runs on the host, so its own local time zone is
not a device offset oracle.

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

`logged-out/profile-pronouns.yaml` is shared by iOS and Android. It writes padded
pronouns to the harness account's real Bluesky profile and checks trimmed
streamer metadata before and after expanding stream info. Its completion hook
restores the original profile even after failure; an originally absent profile
becomes an empty record because the node indexes updates, not deletions.
`js/e2e-web/flows/pronouns.spec.ts` additionally covers blank/missing values and
chat profile cards in both web shells. Native chat-card pronouns use the public
AppView, which does not know the local harness account, so that boundary is
substituted only in the browser suite.

## Invalid handle coverage

`logged-out/invalid-handle.yaml` covers an account whose handle no longer
verifies, while its DID and live stream stay the same. Its `onFlowStart` hook
runs `invalid-handle.js` on the host. The script signs in to the harness PDS as
the harness account and moves it to a fresh `invalid-<base36>.test` handle.
Public resolution can't verify that handle, so when the script asks the node to
`com.atproto.identity.refreshIdentity` the account, the node must report the
`handle.invalid` sentinel.

The flow waits up to 60 seconds for the home card to name the account by its
bare DID. The node's live feed caches identities for 30 seconds, and the app
polls it every 3. The flow then checks that the screen shows no sentinel and no
`@did:` label, opens the stream from that card, and waits for the "Now streaming"
chat system message.

`onFlowComplete` runs even when the flow or its setup fails. It moves the account
back to `ACCOUNT_HANDLE` and requires the node to verify that handle again. It
then relaunches the app and waits for the card to show `@<handle>`, so later
flows see the verified handle.

The flow is shared by iOS and Android. It needs `ACCOUNT_DID`, `ACCOUNT_HANDLE`,
`ACCOUNT_PASSWORD`, `PDS_HTTP_URL`, and `SERVER_HTTP_URL`. `SERVER_HTTP_URL` is
the node's plain-HTTP loopback URL, which is the harness's own `SERVER_URL`.
Maestro's `SERVER_URL` is the HTTPS name the app uses, and the host's JavaScript
doesn't trust the harness CA. The harness must accept `.test` handles on its PDS,
as `js/dev-env` configures it to. To run the flow alone:

```bash
E2E_FLOWS=".maestro/logged-out/invalid-handle.yaml" hack/e2e-local.sh android
```

The Web counterpart, `js/e2e-web/flows/invalid-handle.spec.ts`, covers the Expo
and Vite shells with the same fixture.

## HTTPS, and logging in

The app reaches the harness over HTTPS only: release builds refuse cleartext,
and `setup/oauth-login.yaml` signs in to the harness's own PDS through the real atproto
OAuth flow (the node's OAuth proxy, then the PDS's sign-in and consent pages
in a Chrome Custom Tab on Android or an authentication session on iOS). The
harness serves public names for 127.0.0.1 with a throwaway CA
(see `pkg/cmd/e2e_https.go`). On Android, the runner configures the rooted emulator:

- a hosts file maps the harness's names, and `plc.directory` (the app
  resolves `did:plc` there itself), to the host at 10.0.2.2;
- the CA goes in the user trust store. Chrome trusts that store; the app only
  does when built with `make android-e2e` (`SP_E2E_BUILD` in
  `js/app/app.config.ts`, which also turns off OTA updates so the run tests
  the JS it was built with). Never ship that build.

On iOS, the runner creates a fresh Simulator, installs the CA in that Simulator's
trusted root store, installs the standalone Release `.app`, and pre-grants
notifications. It deletes that Simulator at exit, including its trust store and
account data. It never changes the Mac's hosts file, DNS, or keychain.

### PLC directory override

Frontend DID and OAuth resolution use `EXPO_PUBLIC_PLC_URL`, defaulting to
`https://plc.directory`. Vite also accepts `VITE_PLC_URL`. These are build-time
settings: rebuild the app after changing them.

`make ios-e2e` defaults to
`https://plc.localhost-pds.streamplace.network`; its public DNS resolves to
loopback, so the Simulator needs no hosts override. The runner sets the matching
`E2E_HTTPS_PLC_HOSTNAME`, forwarded as the harness's `--https-plc-hostname`.
For a custom hostname, set `E2E_HTTPS_PLC_HOSTNAME` for **both** the build and run;
it must resolve to 127.0.0.1.

Backend identity resolution already uses `SP_PLC_URL` (the harness supplies its
local PLC endpoint). The pinned oatproxy dependency still hardcodes
`plc.directory`; the harness's process-local CONNECT proxy routes that name to
the same local PLC. Its certificate covers both names. No system proxy or DNS
configuration is required.

## Run it locally

### iOS Simulator (macOS)

Install Xcode with an iOS Simulator runtime, the toolchains pinned in
`mise.toml` (including Node 26 and Go 1.27), CocoaPods, Maestro, and
notification-permission tooling. The iOS CI build uses Xcode 26.6.
The runner prefers Maestro on `PATH`, then falls back to
`~/.maestro/bin/maestro`; put the CI-pinned release on `PATH` for tool parity.

```bash
brew install socat wix/brew/applesimutils
sudo bash hack/e2e-https-macos.sh install   # one-time loopback HTTPS setup
mise exec -- make app
mise exec -- make dev
mise exec -- make ios-e2e # embeds JS, no Metro or signing
mise exec -- bash hack/e2e-local.sh ios
```

`ios-e2e` reuses the `ios-simulator` build used by CI, with the local PLC
directory configured and a versioned `.app` copied into `bin/`.
Use `DEVELOPER_DIR` to select a different installed Xcode if needed.

The runner creates a disposable iPhone, preferring a booted compatible iPhone
and otherwise the newest installed runtime. By default it excludes iOS majors
newer than the selected Xcode SDK. `IOS_SIMULATOR_UDID` explicitly overrides that
selection.

```bash
xcrun simctl list devices available # choose an existing iPhone template
IOS_SIMULATOR_UDID=<existing-iphone-udid> mise exec -- bash hack/e2e-local.sh ios
IOS_SIMULATOR_UDID=<existing-iphone-udid> \
  E2E_FLOWS=".maestro/logged-in/chat-timezone.yaml" mise exec -- bash hack/e2e-local.sh ios
```

`IOS_SIMULATOR_UDID` selects a template; the original device is never erased or
installed into. `IOS_APP` can select a specific e2e `.app`. Rebuild with
`make ios-e2e` after app changes; the runner installs but does not rebuild it.
The build regenerates `js/app/ios`, leaving Android untouched. Never ship e2e
builds.

### Android

```bash
make dev           # harness binary
make android-e2e   # bin/streamplace-*-android-e2e.apk (needs JDK 17 + ANDROID_HOME)
# boot a Google APIs emulator (Play Store images can't be rooted), e.g.
sdkmanager "system-images;android-34;google_apis;x86_64"
avdmanager create avd -n e2e-api34 -k "system-images;android-34;google_apis;x86_64" -d pixel_6
emulator -avd e2e-api34 -no-window -no-audio -no-snapshot-save &
hack/e2e-local.sh android
```

The runner starts the harness, installs the app, prepares the device, runs
the flows and tears the harness down. Screenshots, Maestro's logs and reports
land in `.maestro/artifacts/` (override with `E2E_ARTIFACTS`).

The runner runs on the host, not in the build container: the emulator has to
reach the harness.

### Port 443

OAuth URLs stay portless, so clients must reach the harness on loopback port 443. The harness itself can listen on a high port behind a local forwarder.

#### macOS

`sudo bash hack/e2e-https-macos.sh install` installs
`/Library/LaunchDaemons/network.streamplace.e2e-https.plist`. launchd owns
`127.0.0.1:443` and launches `socat` as `nobody` per connection, forwarding to
`127.0.0.1:38443`. The service persists across reboots, stays loopback-only,
and changes neither DNS nor firewall rules. It reserves local port 443, so do
not install it alongside another service using that listener.

The shared e2e harness detects the loaded service and defaults to port 38443;
`E2E_HTTPS_PORT` still overrides that default. Inspect or remove it with:

```bash
launchctl print system/network.streamplace.e2e-https
sudo bash hack/e2e-https-macos.sh uninstall
```

Relay errors are written to `/var/log/streamplace-e2e-https.log`. If Homebrew
relocates `socat`, rerun the installer to refresh its executable path.

The macOS runner enables Go 1.27's `x509sslcertoverrideplatform` setting for its
own processes, allowing `SSL_CERT_DIR` to supply the throwaway CA instead of
changing the Mac keychain. Those Go processes trust the local harness CA only;
the self-contained suite does not need external HTTPS services.

#### Linux

The harness binds 127.0.0.1:443 by default; without permission, it reports how
to allow that until the next reboot. For a permanent development setup, redirect
loopback 443 to a high port with a systemd unit, and point the harness there:

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

### iOS Simulator (macOS required)

Use the same disposable-Simulator runner for the upstream-station scenario:

```bash
mise exec -- make ios-e2e
SP_E2E_ATMOQ_BIN=/path/to/atmoq \
  mise exec -- bash hack/e2e-local.sh ios --upstream-station
```

The runner configures the fixture PLC hostname, trusts the run's CA only in
the disposable Simulator, and removes that Simulator afterward. No Mac
DNS/hosts override or system-keychain change is needed. The app must allow
local-network transport (`NSAllowsLocalNetworking=true`) for the local
node's loopback HTTP URL; do not enable blanket arbitrary HTTP loads.

## Notes / gotchas

- **macOS open-file limits:** Expo's native resource phase starts a Bash login
  shell even if your interactive shell is Fish. Avoid lowering its hard
  descriptor ceiling in `~/.bashrc`; use a soft-only rule such as
  `ulimit -Sn 10240` (or higher) for large Metro graphs. Check with
  `bash -l -c 'ulimit -Sn; ulimit -Hn'` if a native build reports `EMFILE`.
- **Other adb devices:** maestro lists no Android devices at all ("Device
  emulator-5554 was requested, but it is not connected") while any device
  adb knows about is `unauthorized`, such as a phone plugged in over USB that
  hasn't allowed this computer. Authorize or unplug it, or restart the adb
  server so it leaves USB devices alone:
  `adb kill-server && adb --one-device emulator-5554 start-server`.
- **Android dialogs:** the runner turns off ANR/crash dialogs ("Pixel Launcher
  isn't responding") and the stylus handwriting tutorial, both of which eat
  taps.
- **Android signing:** every build signs with its own throwaway keystore, so the
  runner uninstalls any earlier copy before installing.
- **iOS notification dialog:** the first-launch "would like to send you
  notifications" prompt dims the whole screen and blocks taps. Both iOS runners
  (`hack/e2e-local.sh ios` and `hack/ios-smoke.sh`) pre-grant it with
  `applesimutils` before Maestro starts. Avoid `clearState`, which reinstalls
  the app and re-triggers the prompt.
- **iOS deep-link confirmation:** a fresh Simulator asks "Open in <app>?" on
  the first custom-scheme launch. `server-setup.yaml` accepts that system
  prompt so standalone deep-link scenarios do not depend on another scenario
  running first.
- **iOS accessibility:** the icon-only account link needs its logged-out
  `Log in` label. The login modal's press-intercepting container must not group
  its children, or iOS hides the handle field and buttons. Intentional compound
  controls (settings toggles and stream cards) use `testID` or substring regex.
- **Native chat profiles:** the chat list uses `keyboardShouldPersistTaps="handled"`
  so the first profile tap reaches the handle. The shared dropdown dismisses the
  keyboard on opening and leaves its content ungrouped for accessibility; only
  the draggable sheet handle is an adjustable control. The profile scenario
  exercises opening, dismissal, reopening, and sending afterward without extra
  taps or keyboard workarounds.
- **Stream-title assertions:** the compact metadata button includes its displayed
  title in its accessibility label. Otherwise iOS exposes only `Stream info`,
  and title assertions accidentally depend on the initial system chat message
  remaining on-screen.
- **iOS authentication consent:** a fresh Simulator asks whether the app may
  use the station's website to sign in. The shared OAuth flow taps `Continue`
  on iOS before waiting for the PDS password form.
- **iOS Password AutoFill:** the runner disables it only inside the disposable
  Simulator, using Maestro's documented secure-input workaround. Password fields
  stay secure. The restriction-profile paths are Simulator internals and may need
  updating for a newer iOS runtime; the Mac and existing Simulators are untouched.
  The OAuth flow also declines Safari's optional `Save Password?` prompt with
  `Not Now`.
- **iOS checkbox state:** Maestro reports the custom RN checkbox as
  `checked=false` even when its accessibility value is `checkbox, checked`.
  The VOD scenario checks that native value on iOS and retains Android's
  `checked` assertions and the original timeouts. Native Settings switches
  expose `checked` correctly on both platforms.
