# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app, run against a self-contained
`streamplace e2e` harness (a local PDS and PLC, a Streamplace node and a
looping test stream). `hack/e2e-local.sh android` runs them on an emulator,
and so does the `android-e2e` job in `.github/workflows/build.yaml`. iOS is not
wired up yet. The web suite (`js/e2e-web`, `hack/e2e-web-local.sh`) mirrors
these flows.

Flows run in the order set by `config.yaml`: `00-server-setup` first (it
points the app at the harness), `03-go-live` before `05-oauth-login` (it
checks that a logged-out user is asked to log in), and `05-oauth-login` last.

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
report land in `.maestro/artifacts/`. The harness binds 127.0.0.1:443; if it
can't, it says how to allow it. On a machine that redirects loopback 443
elsewhere (an iptables REDIRECT rule), set `E2E_HTTPS_PORT` to the target.

The runner runs on the host, not in the build container: the emulator has to
reach the harness.

## Notes / gotchas

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
