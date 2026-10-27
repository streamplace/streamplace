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
- `android-debug` and `android-release`: build APKs, AABs, and bundletool `.apks`
  archives directly, without the GitLab registry upload steps.
- `desktop-darwin`: `make desktop-darwin-linux` packages both architectures
  with Electron Forge, signs each `.app` with `rcodesign` in the Forge
  `postPackage` hook, and builds the DMG with `hack/make-dmg.sh` (hfsprogs +
  libdmg-hfsplus). Without `DEVELOPER_CERTIFICATE_BASE64` the app is ad-hoc
  signed; with `APPLE_CODESIGN_API_KEY` it is notarized and stapled.
- `build-ios`: `make ios` on a macOS machine. This is the only job that cannot
  run on Linux, because it needs Xcode. With `MAC_HOST`, `make all` checks the
  remote commit and retrieves the resulting archive into the local `bin/`.
  GitLab's `build-ios` job remains disabled until a macOS runner exists.

The tag-only publishing jobs (`npm-release`, `release`, `deb-release`,
`homebrew-release`) are not run: they need CI credentials and should not fire
by accident.

## Tag checkouts

`make all` detects an exact tag with `git describe --tags --exact-match` and
sets `CI_COMMIT_TAG`, so `make version` produces `vX.Y.Z` instead of a
`vX.Y.Z-<hash>` snapshot and the embedded update URLs use the `latest`
channel. The checkout must be exactly at the tag and clean; one commit past it
is a branch build.

The mobile app's `version`, `runtimeVersion`, and Android `versionCode` come
from `js/app/package.json` at that tag, not from the tag name. Verify the tag
contains the intended release bump:

```sh
git show v0.12.12:js/app/package.json | grep -E '"(version|runtimeVersion)"'
```

`make all` skips iOS unless `MAC_HOST` is set. The remote checkout must be at
the same commit; the script verifies that and passes the tag/branch metadata
into the remote `make version`. If the tag was created after the remote clone,
run `git fetch --tags` there; the script checks that the remote has the tag.

## Publishing releases on ATProto

After configuring a PDS app password, one command builds the supported pipeline,
uploads the release files as PDS blobs, and publishes release records:

```sh
export SP_RELEASE_PDS=https://your-pds.example
export SP_RELEASE_PASSWORD=your-app-password

# Development build: the existing DESC version, published to next.
make release-atproto

# Stable build: run at the commit identified by this existing tag.
make release-atproto RELEASE_VERSION=v0.12.12
```

Build-and-publish requires a clean checkout, including nonignored untracked files,
so the source commit actually identifies the code being built. It reuses
`make all`, including its signing configuration; it does not cut a tag or publish
to npm, Homebrew, or the Debian repository. Stable versions must match their tag;
development versions must contain the source commit's hash prefix and an ancestor
version tag. Stable releases use `latest`; development builds use `next`.
Stable build-and-publish also sets `SP_PRODUCTION_RELEASE=true` for the builder
and remote Mac, selecting Streamplace's production mobile identity (`tv.aquareum`)
instead of the standalone development identity (`tv.aquareum.dev`).

Both the Linux and GitLab pipelines publish 19 standard distributables plus two
bundletool `.apks` archives. Missing required outputs fail before publication. iOS requires
`MAC_HOST` and optionally `MAC_REPO`; without a Mac it is explicitly reported as
skipped, just as with `make all`. `ALL_STRICT=1` makes that skip fatal. Existing
signing credentials and `SIGN_OFFLINE` behavior are unchanged.

To publish already-built or separately signed artifacts without rebuilding:

```sh
make publish-atproto \
  RELEASE_VERSION=v0.12.5 \
  RELEASE_COMMIT=29e8e011091d3a5c1d2aac73a42f85786e0b16e2 \
  RELEASE_ARTIFACTS=/path/to/release-artifacts
```

The source commit and tags must exist locally. A historical commit is supported
in this prebuilt mode; build and CI modes require the producers' checkout commit.
The artifact directory may contain other versions and known build intermediates:
only the exact version's deliverables are staged. Supplied iOS archives and
bundletool archives are included when present. Unknown matching-version files,
missing standard outputs, ambiguous Windows update packages, and invalid registry
checksums fail rather than publishing an incomplete release.

Add `RELEASE_DRY_RUN=1` to preview the records without PDS credentials or network
access to the PDS. This does not skip a requested build or GitLab download.
`RELEASE_CHANNEL` overrides the channel and `RELEASE_CHANGELOG` supplies a canonical
changelog URL. Omit the changelog if none is available; it cannot be added to an
immutable version later.

### Records and account

The default publisher is stream.place,
`did:plc:rbvrr34edl5ddpuwcubjiost`. Override `SP_RELEASE_DID` for another account;
`SP_RELEASE_IDENTIFIER` optionally supplies a different login identifier. The
authenticated DID must equal the publishing DID. Keep credentials in environment
variables, not command-line arguments.

The publisher uses the draft from
[`did:plc:zimrxnxvvkwixvtkazpx3zfx`](https://pds.kart.sh/xrpc/com.atproto.repo.listRecords?repo=did:plc:zimrxnxvvkwixvtkazpx3zfx&collection=com.atproto.lexicon.schema),
pinned in `js/streamplace/scripts/release-schemas.json`. It validates record bodies
and artifact fragments locally before publishing:

- `streamplace` owns node/mobile artifacts; `streamplace-desktop` is its child.
- Each manifest includes a pinned `software.release.defs#gitRevision` and native
  `software.release.defs#blobArtifact` fragments, with runtime targets.
- macOS targets use `macos`; architectures use `amd64` and `arm64`. Android and
  iOS architectures are unspecified rather than inferred from build flavors.
- Version keys are `streamplace.<version>` and
  `streamplace-desktop.<version>`; channel keys similarly use dots, with the
  draft's `channel` field. Manifest keys are TIDs.

Uploads are streamed, verified against local SHA-256 blob CIDs and sizes, and
retain the MIME type returned by the PDS. Once all uploads succeed, projects,
manifests, versions, and channels are written in one atomic repository commit.
Conflicting repository writes fail safely. Identical reruns are no-ops; changing
an already published release fails, and rerunning an older release does not roll
its channel back.

Choose one build/publisher for each version. Independent rebuilds are not
necessarily byte-identical, especially after signing, and cannot replace an
existing immutable release. Reuse its original artifacts for reruns, with the
same optional iOS artifact set.

The PDS must support large release files. The reference PDS defaults to 5 MiB per
blob; configure a suitable limit, for example `PDS_BLOB_UPLOAD_LIMIT=1073741824`,
along with reverse-proxy limits and sufficient storage for multiple gigabytes per
release. An advertised upload limit is checked before any upload.

### GitLab

The tag-only `release-atproto` job waits for every enabled producer, including
Windows, and the test job. Configure `SP_RELEASE_PDS` and `SP_RELEASE_PASSWORD` as
masked/protected CI variables, plus account overrides if needed. The job runs:

```sh
make ci-release-atproto
```

This collects the exact generic-package name and version using `CI_JOB_TOKEN`,
paginates package/file lists, and verifies downloaded sizes and SHA-256 hashes
before using the same publisher as the host command. Registry credentials are
not forwarded across origin-changing redirects. Existing GitLab releases and
other distribution jobs remain enabled.

The lower-level Node publisher can also run directly on a host with workspace
dependencies installed; `node js/streamplace/scripts/release.mjs` prints its
required flags. Unlike the Make wrappers, it publishes the supplied subset of
files and does not enforce the full producer matrix.

Run release regressions against an isolated local PDS with:

```sh
pnpm --filter @atcute/internal-dev-env test:release
```

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

Set `MAC_HOST` to an SSH destination whose checkout is clean and at the same commit:

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
