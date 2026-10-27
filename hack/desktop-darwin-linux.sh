#!/usr/bin/env bash
#
# Build the macOS desktop app (both architectures) on Linux.
#
# Electron Forge can package and make the darwin zip here; the forge config
# signs each .app with rcodesign in its postPackage hook and leaves the DMG to
# hack/make-dmg.sh (MakerDMG needs hdiutil). Signing credentials come from the
# same environment as the Go binaries: DEVELOPER_CERTIFICATE_BASE64,
# DEVELOPER_CERTIFICATE_PASSWORD, and APPLE_CODESIGN_API_KEY for notarization.
# Without a certificate the app is ad-hoc signed, like `make darwin-*`.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"

for tool in rcodesign zip dmg mkfs.hfsplus hfsplus; do
  if ! command -v "$tool" >/dev/null; then
    echo "$tool not found; the builder image is stale (recreate the container" >&2
    echo "after .ci/dockerfile-hash.yaml changes) or this is not the container." >&2
    exit 1
  fi
done

# @electron/fuses resets the ad-hoc signature after flipping fuses by invoking
# `codesign`. There is no Apple codesign here, and the postPackage hook signs
# the bundle with rcodesign afterwards, so make that one call a no-op.
export PATH="$REPO/hack/linux-bin:$PATH"

for arch in amd64 arm64; do
  if [ ! -x "build-darwin-$arch/streamplace" ]; then
    echo "=== building darwin-$arch binary ==="
    make "darwin-$arch"
  fi
done

VERSION="$(go run ./pkg/config/git/git.go -v)"
if [ -z "${DEVELOPER_CERTIFICATE_BASE64:-}" ]; then
  echo "DEVELOPER_CERTIFICATE_BASE64 unset: building an ad-hoc-signed app"
fi

mkdir -p bin

package_arch() {
  local arch="$1" forge_arch zip_file app
  case "$arch" in
    amd64) forge_arch=x64 ;;
    arm64) forge_arch=arm64 ;;
  esac

  echo "=== packaging darwin-$arch ==="
  rm -rf "js/desktop/out/make/zip/darwin/$forge_arch"
  (
    cd js/desktop
    pnpm run make --platform darwin --arch "$forge_arch"
  )

  zip_file="$(find "js/desktop/out/make/zip/darwin/$forge_arch" -maxdepth 1 -name '*.zip' -print -quit)"
  if [ -z "$zip_file" ]; then
    echo "no zip produced for darwin-$arch" >&2
    exit 1
  fi
  mv "$zip_file" "bin/streamplace-desktop-$VERSION-darwin-$arch.zip"

  app="js/desktop/out/Streamplace-darwin-$forge_arch/Streamplace.app"
  hack/make-dmg.sh "$app" "Streamplace $VERSION" \
    "bin/streamplace-desktop-$VERSION-darwin-$arch.dmg"
}

pids=()
for arch in amd64 arm64; do
  package_arch "$arch" &
  pids+=("$!")
done
for pid in "${pids[@]}"; do
  if ! wait "$pid"; then
    for other in "${pids[@]}"; do
      kill "$other" 2>/dev/null || true
    done
    wait || true
    exit 1
  fi
done
