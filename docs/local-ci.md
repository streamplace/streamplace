# Running the GitLab pipeline locally

`make all` runs the build jobs from `.gitlab-ci.yml` in the pinned builder
container, in the same order the CI jobs produce them, and leaves artifacts in
`bin/`.

```sh
make all
```

Run it from a Linux host (or WSL2) with Docker. On the host it calls
`hack/container.sh`; inside the builder container it runs in place. The script
is `hack/all.sh`.

## What runs

- `test`: `make dev`, the Go test suite with `STREAMPLACE_TEST_COUNT=1`, and a
  lexicon regeneration check. The check fails only if running `make lexicons`
  changes the tree, so it also works with unrelated local edits (CI's
  `make ci-lexicons` compares the whole diff). The test build uses
  `build-test-linux-amd64` so that it does not share a build directory with the
  static release build.
- `linux-amd64`, `linux-arm64`, `windows-amd64`, `darwin-amd64`,
  `darwin-arm64`: the node build, `test-vod` (where the binary can run),
  `desktop-$TARGET`, and `pkg-$TARGET`.
- `android-debug` and `android-release`: `make ci-android-debug` and
  `make ci-android-release`.
- `desktop-darwin`: `make desktop-darwin-linux` packages both architectures
  with Electron Forge, signs each `.app` with `rcodesign` in the Forge
  `postPackage` hook, and builds the DMG with `hack/make-dmg.sh` (hfsprogs +
  libdmg-hfsplus). Without `DEVELOPER_CERTIFICATE_BASE64` the app is ad-hoc
  signed; with `APPLE_CODESIGN_API_KEY` it is notarized and stapled.
- `build-ios`: `make ios` on a macOS machine. This is the only job that cannot
  run on Linux, because it needs Xcode.

The tag-only publishing jobs (`npm-release`, `release`, `deb-release`,
`homebrew-release`) are not run: they need CI credentials and should not fire
by accident.

## Building and signing separately

The macOS artifacts need the Developer ID certificate after they are compiled:
both the Go binary and every Mach-O inside the Electron app gain a signature.
To keep the certificate off the build machine:

```sh
# Big machine (builder container):
make build-signable

# Signing machine (certificate + rcodesign):
make sign SIGN_INPUT=dist/streamplace-<version>-sign-input.tar.gz
```

`build-signable` compiles the two darwin binaries with `SKIP_SIGN=1`, packages
both Electron `.app` bundles with `STREAMPLACE_SKIP_SIGNING=1`, and writes
`dist/streamplace-<version>-sign-input.tar.gz` plus a `.sha256`. The tarball
contains the unsigned binaries, unsigned app bundles, `VERSION`, `COMMIT`, a
`SHA256SUMS` file, and a `SYMLINKS` listing. The build machine does not need
any Apple credentials.

`build-signable` needs the pinned builder container (Go, Node/pnpm, Meson,
Rust, osxcross) and nothing else: no prior `make dev`, no Apple credentials,
and no macOS. `sign` is the opposite: it needs `rcodesign`, `zip`, and the
host DMG tool (`hdiutil`, or hfsprogs/libdmg-hfsplus on Linux), but not the
build toolchain.

`sign` verifies `SHA256SUMS` and `SYMLINKS`, signs each binary and app with
`rcodesign`, optionally notarizes and staples, and writes the release artifacts
to `bin/`:

- `streamplace-<version>-darwin-{amd64,arm64}.tar.gz`
- `streamplace-desktop-<version>-darwin-{amd64,arm64}.{zip,dmg}`
- `streamplace-<version>-SHA256SUMS`

Without `SIGN_INPUT`, `make sign` signs the in-place `build-darwin-*` and
`js/desktop/out` trees (that mode needs Go to compute the version). The signing
machine needs the checkout for `hack/` and `js/desktop/entitlements.plist`.

Signing offline is supported with `SIGN_OFFLINE=1`. Note that Apple notarization
requires a secure timestamp from Apple's timestamp server, so an offline
signature is **not** notarizable without re-signing with the certificate while
online. If the signing machine can reach Apple, leave `SIGN_OFFLINE` unset and
set `APPLE_CODESIGN_API_KEY` to notarize in the same pass.

Relevant environment:

- `DEVELOPER_CERTIFICATE_BASE64`, `DEVELOPER_CERTIFICATE_PASSWORD` — required
  for a real signature; `ALLOW_ADHOC=1` permits an ad-hoc build instead.
- `APPLE_CODESIGN_API_KEY` — App Store Connect API key JSON, enables
  notarization (`SIGN_NOTARIZE=auto` by default).
- `SIGN_NOTARIZE=0|1|auto` — override notarization.
- `SIGN_TIMESTAMP_URL` — rcodesign timestamp server; `none` disables it.
- `SIGN_OFFLINE=1` — implies `SIGN_TIMESTAMP_URL=none` and `SIGN_NOTARIZE=0`.

`make all` still builds and signs in one go on the build machine; the split is
for moving the certificate to a separate environment.

## iOS on a macOS machine

Set `MAC_HOST` to an SSH destination whose checkout is at the same commit:

```sh
MAC_HOST=admin@mac-mini MAC_REPO=/Users/admin/streamplace make all
```

The remote needs the same toolchain the CI macOS job installs (Go, Node 22,
Meson, Rust, Xcode 26). Without `MAC_HOST`, `make all` prints the skipped iOS
job and exits successfully; set `ALL_STRICT=1` to make that a failure.

There is no way to build the iOS archive on Linux or Windows: `xcodebuild`
and the iOS SDK only exist on macOS. On an AMD/Threadripper host, the practical
option is a macOS VM (OSX-KVM/OpenCore with the AMD kernel patches) or a
physical/cloud Mac. Xcode 26.3 supports macOS Sequoia 15.6 and later, so an
x86_64 VM is sufficient; the archive is built with `CODE_SIGNING_ALLOWED=NO`,
so no Apple account is needed for the build itself.

## Notes

- A fresh checkout is assumed, like a CI runner. If `build-linux-amd64` was
  configured for the shared dev build, `make all` reconfigures it for the
  static release build. Run the release targets manually with a different
  `BUILDDIR` if you need to keep that dev build intact.
- The builder image must be the one pinned in `.ci/dockerfile-hash.yaml`.
  `hack/container.sh` warns if an existing container runs a different image;
  recreate it after the pin changes.
