# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app, run against a self-contained
`streamplace e2e` harness (a local PDS and PLC, a Streamplace node and a
looping test stream). `hack/e2e-local.sh android` runs them on an emulator,
and so does the `android-e2e` job in `.github/workflows/build.yaml`. iOS is not
wired up yet. The web suite (`js/e2e-web`, `hack/e2e-web-local.sh`) mirrors
these flows.

Flows run in the order set by `config.yaml`: `00-server-setup` first (it
points the app at the harness), then the flows that expect a logged-out app,
then `05-oauth-login`, then the flows that expect a logged-in one
(`06-chat-reply`). There is no `03`: native builds hide the Go Live controls
that `03-go-live` covered, and `02-tabs` checks they stay hidden. The web
suite still has its `03-go-live`.

## HTTPS, and logging in

The app reaches the harness over HTTPS only: release builds refuse cleartext,
and `05-oauth-login` signs in to the harness's own PDS through the real atproto
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
