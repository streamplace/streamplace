# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app. This branch carries the flows
themselves; the mobile runner (`hack/e2e-local.sh`) and the CI jobs that drive
them (`android-e2e`, `ios-e2e` in `.github/workflows/build.yaml`) still live on
the `natb/e2e` branches and have not landed on `next`. Until they do, nothing
runs these automatically — invoking Maestro by hand is the only way here. The
web suite (`js/e2e-web`, `hack/e2e-web-local.sh`) is local-only too; AGENTS.md
§5 covers it and `make provision` runs it.

Flows run in the order set by `config.yaml`: `00-server-setup` must go first
(it points the app at the self-contained test server), and `03-go-live` last
(it leaves the login modal open). `05-custom-domain` publishes a brand
record for a custom domain the harness granted, points the app at the node
under that hostname, checks Settings → About names the node by that brand,
and points the app back at `SERVER_URL`.

## Run it locally

Without the runner, start the harness yourself and point Maestro at it:

```bash
make dev                              # harness binary
./build-linux-amd64/streamplace e2e --custom-domain localhost
                                      # SERVER_URL, ACCOUNT_*, PDS_URL on stdout
adb reverse tcp:<port> tcp:<port>     # Android: device localhost -> harness
maestro test -e APP_ID=tv.aquareum.dev \
  -e SERVER_URL=http://10.0.2.2:<port> \
  -e CUSTOM_DOMAIN_URL=http://localhost:<port> \
  -e PDS_URL=<pds-url> -e ACCOUNT_HANDLE=<handle> \
  -e ACCOUNT_DID=<did> -e ACCOUNT_PASSWORD=<password> .maestro/
```

`CUSTOM_DOMAIN_URL` is the same node under the granted hostname. On the iOS
simulator it is `http://localhost:<port>` as-is (with `SERVER_URL`
`http://127.0.0.1:<port>`). The flow's scripts run on the Maestro host, so
`PDS_URL` needs no rewriting.

The upstream runner does all of that — it starts the harness (local PDS/PLC +
a looping test stream), installs the app, prepares the device, runs the flows
and tears the harness down. Either way it only _runs_ — build the app first:

- **harness binary:** `make dev`
- **android APK:** `make android-release` (needs JDK 17 + `ANDROID_HOME`)
- **ios sim app** (needs `watchman`, `applesimutils`, `maestro`, `idb`):
  ```bash
  cd js/app && CI=true pnpm run build
  cd ios && pod install && cd ../../..
  make dev   # re-embeds the fresh web bundle into the harness binary
  xcodebuild -workspace js/app/ios/Streamplace.xcworkspace -scheme Streamplace \
    -configuration Release -sdk iphonesimulator -derivedDataPath js/app/ios/build \
    -destination 'generic/platform=iOS Simulator' build
  ```

## Notes / gotchas

- **iOS notification dialog:** the first-launch "would like to send you
  notifications" prompt dims the whole screen and blocks taps. It only shows
  on a fresh install, so the run pre-grants the permission with
  `applesimutils` and avoids `clearState` (which reinstalls and re-triggers
  it).
- **iOS accessibility collapse:** iOS merges tappable containers (the settings
  toggle, the stream cards, the login modal) into a single accessibility
  element, so those are matched by `testID` or substring regex, not exact text.
- **Android emulator** must be reachable at `10.0.2.2` (the script rewrites the
  harness URL). The iOS simulator shares the host loopback, so no rewrite.
