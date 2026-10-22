#!/usr/bin/env bash
#
# Sign the macOS inputs produced by `make build-signable` and write the final
# release artifacts to bin/:
#
#   make sign SIGN_INPUT=dist/streamplace-<version>-sign-input.tar.gz
#
# With no SIGN_INPUT, signs the in-place build tree (build-darwin-*/streamplace
# and js/desktop/out/Streamplace-darwin-*/Streamplace.app).
#
# This machine needs rcodesign and, for the DMG, either hdiutil (macOS) or
# hfsprogs/hfsplus/libdmg-hfsplus (Linux builder image). It does not need Go,
# Node, Meson, or the rest of the build toolchain when SIGN_INPUT is used.
#
# Credentials (see hack/rcodesign-app.sh and util/osxcross-codesign.sh):
#   DEVELOPER_CERTIFICATE_BASE64, DEVELOPER_CERTIFICATE_PASSWORD
#   APPLE_CODESIGN_API_KEY   notarization; requires a timestamp server
#   SIGN_TIMESTAMP_URL       "none" for a truly offline sign
#   SIGN_OFFLINE=1           sign without timestamp/notarization
#   ALLOW_ADHOC=1            allow an unsigned/ad-hoc release
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
INPUT="${1:-${SIGN_INPUT:-}}"
WORK=""
OUT_DIR="$REPO/bin"

cleanup() {
  if [ -n "$WORK" ]; then rm -rf "$WORK"; fi
}
trap cleanup EXIT

if [ -z "${DEVELOPER_CERTIFICATE_BASE64:-}" ] && [ "${ALLOW_ADHOC:-0}" != "1" ]; then
  echo "DEVELOPER_CERTIFICATE_BASE64 is not set; set ALLOW_ADHOC=1 for an ad-hoc build" >&2
  exit 1
fi

if command -v sha256sum >/dev/null; then
  HASH=(sha256sum)
else
  HASH=(shasum -a 256)
fi

if [ -n "$INPUT" ]; then
  if [ ! -f "$INPUT" ]; then
    echo "$INPUT not found" >&2
    exit 1
  fi
  WORK="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-sign.XXXXXX")"
  tar -xzf "$INPUT" -C "$WORK"
  SIGN_ROOT="$WORK"

  echo "=== verifying $INPUT ==="
  if [ "${HASH[0]}" = "sha256sum" ]; then
    ( cd "$SIGN_ROOT" && sha256sum -c SHA256SUMS )
  else
    ( cd "$SIGN_ROOT" && shasum -a 256 -c SHA256SUMS )
  fi
  (
    cd "$SIGN_ROOT"
    find . -type l -print |
      LC_ALL=C sort |
      while IFS= read -r l; do printf '%s -> %s\n' "$l" "$(readlink "$l")"; done >SYMLINKS.actual
    diff -u SYMLINKS SYMLINKS.actual
    rm -f SYMLINKS.actual
  )
  VERSION="$(cat "$SIGN_ROOT/VERSION")"
  BUILD_COMMIT="$(cat "$SIGN_ROOT/COMMIT")"
  if [ "$BUILD_COMMIT" != "$(git rev-parse HEAD)" ]; then
    echo "warning: input was built at $BUILD_COMMIT but this checkout is at $(git rev-parse HEAD)" >&2
    echo "warning: signing uses this checkout's entitlements and hack/ scripts" >&2
  fi
else
  SIGN_ROOT="$REPO"
  VERSION="$(go run ./pkg/config/git/git.go -v)"
fi

mkdir -p "$OUT_DIR"
ARTIFACTS=()

for arch in amd64 arm64; do
  case "$arch" in
    amd64) forge_arch=x64 ;;
    arm64) forge_arch=arm64 ;;
  esac

  if [ -n "$INPUT" ]; then
    BIN="$SIGN_ROOT/darwin-$arch/streamplace"
    APP="$SIGN_ROOT/darwin-$arch/Streamplace.app"
  else
    BIN="$SIGN_ROOT/build-darwin-$arch/streamplace"
    APP="$SIGN_ROOT/js/desktop/out/Streamplace-darwin-$forge_arch/Streamplace.app"
  fi
  if [ ! -x "$BIN" ]; then
    echo "$BIN missing; run make build-signable first" >&2
    exit 1
  fi
  if [ ! -d "$APP" ]; then
    echo "$APP missing; run make build-signable first" >&2
    exit 1
  fi

  echo "=== signing darwin-$arch ==="
  SIGN_NOTARY_WAIT=1 util/osxcross-codesign.sh "$BIN"
  tar -C "$(dirname "$BIN")" \
    -czf "$OUT_DIR/streamplace-$VERSION-darwin-$arch.tar.gz" ./streamplace
  ARTIFACTS+=("streamplace-$VERSION-darwin-$arch.tar.gz")

  hack/rcodesign-app.sh "$APP"
  (
    cd "$(dirname "$APP")"
    # zip updates an existing archive instead of replacing it, so remove the
    # previous build's entries before signing a rebuilt app.
    rm -f "$OUT_DIR/streamplace-desktop-$VERSION-darwin-$arch.zip"
    zip -qry "$OUT_DIR/streamplace-desktop-$VERSION-darwin-$arch.zip" \
      "$(basename "$APP")"
  )
  ARTIFACTS+=("streamplace-desktop-$VERSION-darwin-$arch.zip")

  hack/make-dmg.sh "$APP" "Streamplace $VERSION" \
    "$OUT_DIR/streamplace-desktop-$VERSION-darwin-$arch.dmg"
  ARTIFACTS+=("streamplace-desktop-$VERSION-darwin-$arch.dmg")
done

CHECKSUMS="$OUT_DIR/streamplace-$VERSION-SHA256SUMS"
(
  cd "$OUT_DIR"
  "${HASH[@]}" "${ARTIFACTS[@]}" >"$(basename "$CHECKSUMS")"
)

echo
echo "signed artifacts in $OUT_DIR:"
cat "$CHECKSUMS"
