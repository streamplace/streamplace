#!/usr/bin/env bash
#
# Build the macOS inputs for the split build/sign workflow:
#
#   make build-signable
#
# Produces unsigned darwin binaries and unsigned Electron .app bundles, then
# packs them into dist/streamplace-<version>-sign-input.tar.gz. Carry that file
# (and this checkout at the recorded commit) to the machine with the Developer
# ID certificate, then run `make sign SIGN_INPUT=...` there.
#
# Nothing here needs signing credentials.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
OUT="$REPO/dist/sign-input"

if [ "$(uname -s)" != "Linux" ]; then
  echo "build-signable runs the Linux build; use the big machine or its container." >&2
  exit 1
fi

. hack/lib/container.sh

step() {
  printf '\n==> %s\n' "$*"
}

step "version and frontends"
run make version
run make app

step "darwin binaries (unsigned)"
for arch in amd64 arm64; do
  run make "darwin-$arch" SKIP_SIGN=1
done

step "desktop app bundles (unsigned)"
for arch in amd64 arm64; do
  case "$arch" in
    amd64) forge_arch=x64 ;;
    arm64) forge_arch=arm64 ;;
  esac
  run bash -c "cd '$REPO/js/desktop' && PATH='$REPO/hack/linux-bin:'\$PATH STREAMPLACE_SKIP_SIGNING=1 pnpm run package --platform darwin --arch '$forge_arch'"
done

step "assemble $OUT"
VERSION="$(run go run ./pkg/config/git/git.go -v)"
COMMIT="$(run git rev-parse HEAD)"
rm -rf "$OUT"
mkdir -p "$OUT"
for arch in amd64 arm64; do
  case "$arch" in
    amd64) forge_arch=x64 ;;
    arm64) forge_arch=arm64 ;;
  esac
  mkdir -p "$OUT/darwin-$arch"
  cp "build-darwin-$arch/streamplace" "$OUT/darwin-$arch/streamplace"
  cp -a "js/desktop/out/Streamplace-darwin-$forge_arch/Streamplace.app" \
    "$OUT/darwin-$arch/Streamplace.app"
done
printf '%s\n' "$VERSION" >"$OUT/VERSION"
printf '%s\n' "$COMMIT" >"$OUT/COMMIT"

# Standard checksums cover regular files; symlinks are listed separately. The
# signing side verifies both before it signs anything.
(
  cd "$OUT"
  find . -type f ! -name SHA256SUMS ! -name SYMLINKS -print |
    LC_ALL=C sort |
    while IFS= read -r f; do sha256sum "$f"; done >SHA256SUMS
  find . -type l -print |
    LC_ALL=C sort |
    while IFS= read -r l; do printf '%s -> %s\n' "$l" "$(readlink "$l")"; done >SYMLINKS
)

step "transport tarball"
mkdir -p "$REPO/dist"
TARBALL="$REPO/dist/streamplace-$VERSION-sign-input.tar.gz"
tar -C "$OUT" -czf "$TARBALL" .
sha256sum "$TARBALL" >"$TARBALL.sha256"

echo
echo "unsigned inputs: $OUT"
echo "transport:       $TARBALL"
cat "$TARBALL.sha256"
