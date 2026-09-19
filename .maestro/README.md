# e2e tests (Maestro)

Cross-platform Maestro flows for the mobile app, run in CI (`android-e2e`
and `ios-e2e` in `.github/workflows/build.yaml`) and locally.

Flows run in the order set by `config.yaml`: `00-server-setup` must go first
(it points the app at the self-contained test server), and `03-go-live` last
(it leaves the login modal open).

## Run it locally

```bash
hack/e2e-local.sh android   # on Linux/macOS, with an emulator running
hack/e2e-local.sh ios       # on macOS
```

The script starts a `streamplace e2e` harness (local PDS/PLC + a looping test
stream), installs the app, prepares the device, runs the flows, and tears the
harness down. It only _runs_ — build the app first:

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
