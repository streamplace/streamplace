#!/usr/bin/env bash
# seed-build-dir.sh — make a fresh worktree Go-test-ready in seconds instead of
# a ~1h cold `make dev-setup`.
#
# The dev (shared-library) build installs everything into the build dir itself
# (meson prefix == build dir). The cgo deps that `go test` links against —
# GStreamer, FFmpeg, glib, x264, libiroh_streamplace — are a pure function of
# the meson inputs (meson.build, subprojects/*.wrap, rust/, Cargo.lock), NOT of
# the branch's Go code. So a new worktree can copy the installed tree from an
# already-built checkout, rewrite the absolute prefix paths in the .pc files,
# and immediately run `go build` / `go test` against it.
#
# Usage:
#   hack/seed-build-dir.sh [DONOR_CHECKOUT] [--force]
#
# DONOR_CHECKOUT defaults to the main checkout this worktree was created from
# (derived from `git rev-parse --git-common-dir`). The donor is only ever read.
#
# After seeding, run tests with hack/test-env.sh, e.g.:
#   hack/test-env.sh go test -run TestStreamTranscoder -v ./pkg/media
#
# What this does NOT cover:
# - Changes to the C/rust dependency stack itself (meson.build, subprojects,
#   rust/, Cargo.lock). The manifest check below catches those and tells you to
#   run a real `make dev-setup`.
# - Changes to the meson option blocks in the Makefile (BASE_OPTS/SHARED_OPTS).
#   Those are rare and not covered by the manifest; if you're editing them,
#   you're doing a real build anyway.
# - The real frontend. js/app/dist gets a placeholder index.html so that
#   packages embedding js/app compile; run `make app` if you need the actual UI.
set -euo pipefail

FORCE=0
DONOR_ROOT=""
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//' | head -30; exit 0 ;;
    *) DONOR_ROOT="$arg" ;;
  esac
done

WT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

BUILDOS="${BUILDOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
BUILDARCH="${BUILDARCH:-$(uname -m | tr '[:upper:]' '[:lower:]')}"
case "$BUILDARCH" in
  aarch64) BUILDARCH=arm64 ;;
  x86_64) BUILDARCH=amd64 ;;
esac
BUILDDIR="${BUILDDIR:-build-$BUILDOS-$BUILDARCH}"

if [ -z "$DONOR_ROOT" ]; then
  COMMON_DIR="$(cd "$WT_ROOT" && git rev-parse --path-format=absolute --git-common-dir)"
  DONOR_ROOT="$(dirname "$COMMON_DIR")"
fi
DONOR_ROOT="$(cd "$DONOR_ROOT" && pwd)"

if [ "$DONOR_ROOT" = "$WT_ROOT" ]; then
  echo "error: donor checkout resolves to this checkout ($WT_ROOT)." >&2
  echo "Pass the path to an already-built checkout explicitly:" >&2
  echo "  hack/seed-build-dir.sh /path/to/built/streamplace" >&2
  exit 1
fi

DONOR_BUILD="$DONOR_ROOT/$BUILDDIR"
DEST_BUILD="$WT_ROOT/$BUILDDIR"

if [ ! -f "$DONOR_BUILD/lib/pkgconfig/streamplacedeps.pc" ]; then
  echo "error: $DONOR_BUILD doesn't look like a completed dev build" >&2
  echo "(missing lib/pkgconfig/streamplacedeps.pc). Run 'make dev-setup' in the" >&2
  echo "donor checkout first, or pass a different donor." >&2
  exit 1
fi

if [ -e "$DEST_BUILD" ]; then
  if [ "$FORCE" = 1 ]; then
    echo "--force: removing existing $DEST_BUILD"
    rm -rf "$DEST_BUILD"
  else
    echo "error: $DEST_BUILD already exists; rm -rf it or pass --force." >&2
    exit 1
  fi
fi

# Hash the inputs that determine the native dependency build. If these differ
# between donor and worktree, the donor's libs may not match what this branch
# expects and you need a real build.
# Only tracked files count: builds promote extra gitignored .wrap files into
# subprojects/, which must not make two otherwise-identical checkouts mismatch.
manifest() {
  local root="$1" out
  out="$(
    cd "$root"
    git ls-files -z -- meson.build 'subprojects/*.wrap' Cargo.toml Cargo.lock rust |
      LC_ALL=C sort -z | xargs -0 -r sha256sum
  )"
  if [ -z "$out" ]; then
    echo "error: git ls-files found no native build inputs in $root" >&2
    echo "(not a streamplace checkout, or git can't read it here?)" >&2
    return 1
  fi
  printf '%s' "$out" | sha256sum | cut -d' ' -f1
}

DONOR_MANIFEST="$(manifest "$DONOR_ROOT")"
WT_MANIFEST="$(manifest "$WT_ROOT")"
if [ "$DONOR_MANIFEST" != "$WT_MANIFEST" ]; then
  echo "error: native build inputs differ between this checkout and the donor:" >&2
  echo "  donor    ($DONOR_ROOT): $DONOR_MANIFEST" >&2
  echo "  worktree ($WT_ROOT): $WT_MANIFEST" >&2
  echo "meson.build / subprojects/*.wrap / rust/ / Cargo.lock changed, so the" >&2
  echo "donor's libraries may not match this branch. Run 'make dev-setup' for a" >&2
  echo "real build, or re-run with --force if you know the change is irrelevant." >&2
  if [ "$FORCE" != 1 ]; then
    exit 1
  fi
  echo "--force: continuing anyway." >&2
fi

echo "Seeding $DEST_BUILD from $DONOR_BUILD ..."
mkdir -p "$DEST_BUILD"
# --reflink=auto: instant CoW copy on filesystems that support it, plain copy
# otherwise. Deliberately NOT hardlinks: a later `meson install` in the donor
# overwrites files in place, which would corrupt hardlinked copies.
for d in lib include libexec bin; do
  if [ -e "$DONOR_BUILD/$d" ]; then
    cp -a --reflink=auto "$DONOR_BUILD/$d" "$DEST_BUILD/$d"
  fi
done

# The .pc files carry the donor's absolute prefix; retarget them here.
grep -rlF --include='*.pc' "$DONOR_ROOT" "$DEST_BUILD" | while read -r pc; do
  sed -i "s|$DONOR_ROOT|$WT_ROOT|g" "$pc"
done

# Placeholder frontend + lexicon stamp so `go build` (go:embed js/app/dist) and
# `make dev` work without a pnpm build. Run `make app` for the real UI.
mkdir -p "$WT_ROOT/js/app/dist" "$WT_ROOT/.build"
[ -f "$WT_ROOT/js/app/dist/index.html" ] || touch "$WT_ROOT/js/app/dist/index.html"
touch "$WT_ROOT/.build/lexicon-stamp"

# Canary: the donor's libs must be loadable in THIS environment. A build dir
# made on the host doesn't load inside the builder container (newer glibc), and
# vice versa can't be assumed either — catch that now, not at `go test` time.
if [ -x "$DEST_BUILD/bin/gio" ]; then
  if ! LD_LIBRARY_PATH="$DEST_BUILD/lib" "$DEST_BUILD/bin/gio" version >/dev/null 2>&1; then
    echo "error: the seeded libraries don't load in this environment:" >&2
    LD_LIBRARY_PATH="$DEST_BUILD/lib" "$DEST_BUILD/bin/gio" version 2>&1 | head -3 >&2 || true
    echo "The donor ($DONOR_BUILD) was probably built against a different libc" >&2
    echo "(e.g. built on the host, but you're seeding inside the builder" >&2
    echo "container). Pick a donor checkout whose build ran in the same" >&2
    echo "environment you'll run tests in." >&2
    rm -rf "$DEST_BUILD"
    exit 1
  fi
fi

{
  echo "seeded-from: $DONOR_BUILD"
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "manifest: $WT_MANIFEST"
} > "$DEST_BUILD/.seeded-from"

echo "Done. Run Go tests with:"
echo "  hack/test-env.sh go test -run <TestName> -v ./pkg/<pkg>"
