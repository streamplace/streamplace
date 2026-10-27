#!/usr/bin/env bash
#
# Run the GitLab build pipeline on this machine, in the pinned builder
# container:
#
#   make all
#
# This mirrors the jobs in .gitlab-ci.yml: the test job, every cross-compiled
# node target (with its desktop package), both Android APKs, the macOS desktop
# app, and iOS. Artifacts land in bin/ as the CI jobs leave them; the iOS
# archive stays on the macOS machine that builds it.
#
# The tag-only release/upload jobs (npm, GitLab release, deb, Homebrew) are
# deliberately not run: they publish and need CI credentials.
#
# iOS is the one job that cannot run on Linux. Set MAC_HOST (and optionally
# MAC_REPO) to dispatch it to a macOS machine with the same checkout; otherwise
# it is reported as skipped. Set ALL_STRICT=1 to make skips fatal.
#
# A fresh checkout is assumed, like a CI runner. `make all` reconfigures
# build-linux-amd64 for the static release build; the test job uses
# build-test-linux-amd64 so the two native builds do not fight.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
TEST_BUILDDIR="${TEST_BUILDDIR:-build-test-linux-amd64}"
SKIPPED=()

step() {
  printf '\n==> %s\n' "$*"
}

if [ "$(uname -s)" != "Linux" ]; then
  echo "make all runs the Linux pipeline; run it on Linux (or in the builder container)." >&2
  exit 1
fi

# GitLab always defines one of these; a local checkout does not. Derive the
# same value so `make version` can build the update URLs outside CI. An exact
# tag wins over a preset branch so `git checkout vX.Y.Z` builds a release.
if [ -z "${CI_COMMIT_TAG:-}" ]; then
  if tag="$(git describe --tags --exact-match 2>/dev/null)"; then
    export CI_COMMIT_TAG="$tag"
  elif [ -z "${CI_COMMIT_BRANCH:-}" ]; then
    export CI_COMMIT_BRANCH="$(git symbolic-ref --short HEAD 2>/dev/null || git rev-parse --short HEAD)"
  fi
fi

. hack/lib/container.sh

# ---------------------------------------------------------------------------
# test job
# ---------------------------------------------------------------------------
step "version and frontends"
run make version
# Build the embedded bundles from the current source; `make dev` would reuse a
# stale dist directory left by a previous branch.
run make app

step "test (dev build and Go tests)"
# Separate build directory: the release target below wants a static
# build-linux-amd64, while dev/tests want a shared one.
run make dev BUILDDIR="$TEST_BUILDDIR"
run env STREAMPLACE_TEST_COUNT=1 make dev-test BUILDDIR="$TEST_BUILDDIR"

step "lexicons are up to date"
# `make ci-lexicons` compares the whole tree with git, which fails on any
# unrelated local edit. Check instead that running the generator does not
# change the tree, which is the invariant CI is protecting.
tree_state() {
  { git diff --binary; git status --porcelain; } | git hash-object --stdin
}
LEXICON_STATE="$(tree_state)"
run make lexicons
if [ "$(tree_state)" != "$LEXICON_STATE" ]; then
  echo "lexicons are out of date; run make lexicons and commit the generated files" >&2
  git diff
  exit 1
fi

# ---------------------------------------------------------------------------
# node jobs: build the target, then its desktop package and distro package
# ---------------------------------------------------------------------------
target() {
  local target="$1"
  step "$target"
  run make "$target"
  # test-vod only runs on the build host, so in CI it is a no-op for the
  # cross-compiled jobs. Here the linux-amd64 binary exists, so run it once.
  if [ "$target" = "linux-amd64" ]; then
    run make test-vod
  fi
  run make "desktop-$target"
  run make "pkg-$target"
}

target linux-amd64
target linux-arm64
target windows-amd64
target darwin-amd64
target darwin-arm64

# ---------------------------------------------------------------------------
# android jobs
# ---------------------------------------------------------------------------
# `make app` above already ran `expo prebuild`, so build the bundles directly.
# The ci-android-* targets also upload to the GitLab package registry, which
# needs CI_JOB_TOKEN and fails on a local machine.
step "android-debug"
run make android-debug

step "android-release"
run make android-release

# ---------------------------------------------------------------------------
# macOS desktop app (packages and signs with rcodesign on Linux)
# ---------------------------------------------------------------------------
step "desktop-darwin"
run make desktop-darwin-linux

# ---------------------------------------------------------------------------
# iOS: requires Xcode, so it stays on macOS
# ---------------------------------------------------------------------------
if [ -n "${MAC_HOST:-}" ]; then
  step "ios on $MAC_HOST"
  COMMIT="$(git rev-parse HEAD)"
  MAC_REPO="${MAC_REPO:-$REPO}"
  # The remote runs `make version` directly, so it needs the same CI metadata
  # the local wrapper derived; otherwise a tag checkout panics there.
  REMOTE_CI=""
  REMOTE_TAG_CHECK=""
  if [ -n "${CI_COMMIT_TAG:-}" ]; then
    REMOTE_CI="CI_COMMIT_TAG='$CI_COMMIT_TAG'"
    REMOTE_TAG_CHECK="git rev-parse -q --verify 'refs/tags/$CI_COMMIT_TAG' >/dev/null || { echo 'remote checkout is missing tag $CI_COMMIT_TAG; run git fetch --tags there' >&2; exit 1; };"
  fi
  if [ -n "${CI_COMMIT_BRANCH:-}" ]; then
    REMOTE_CI="$REMOTE_CI CI_COMMIT_BRANCH='$CI_COMMIT_BRANCH'"
  fi
  # The remote checkout must be at the same commit; make all does not push.
  ssh "$MAC_HOST" "cd '$MAC_REPO' && test \"\$(git rev-parse HEAD)\" = '$COMMIT' && $REMOTE_TAG_CHECK $REMOTE_CI make version install app ios"
else
  SKIPPED+=("build-ios: needs macOS; set MAC_HOST=user@host (and MAC_REPO=/path/to/checkout)")
fi

# ---------------------------------------------------------------------------
# report
# ---------------------------------------------------------------------------
printf '\n'
if [ "${#SKIPPED[@]}" -gt 0 ]; then
  printf 'skipped (cannot run on this host):\n'
  printf '  - %s\n' "${SKIPPED[@]}"
  if [ "${ALL_STRICT:-0}" = "1" ]; then
    exit 1
  fi
fi
echo "make all: done"
