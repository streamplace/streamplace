#!/usr/bin/env bash
# Build + publish, publish an existing artifact directory, or collect the exact
# GitLab package version + publish. See the release-atproto Make targets.
set -euo pipefail
cd "$(dirname "$0")/.."
REPO="$PWD"
MODE="${1:-publish}"
case "$MODE" in build|publish|ci) ;; *) echo "unknown release mode: $MODE" >&2; exit 1 ;; esac

# Check before starting the builder or an expensive make all. Secrets stay in
# the environment, not Make recipes, CLI arguments, or diagnostic output.
if [ "${RELEASE_DRY_RUN:-0}" != "1" ]; then
  : "${SP_RELEASE_PDS:?set SP_RELEASE_PDS to the publishing PDS URL}"
  : "${SP_RELEASE_PASSWORD:?set SP_RELEASE_PASSWORD to the publishing app password}"
fi
if [ "$MODE" = ci ]; then
  : "${CI_API_V4_URL:?CI_API_V4_URL is required}"
  : "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
  : "${CI_JOB_TOKEN:?CI_JOB_TOKEN is required}"
  if [ -z "${CI_COMMIT_TAG:-}${CI_COMMIT_BRANCH:-}" ]; then
    echo "CI_COMMIT_TAG or CI_COMMIT_BRANCH is required" >&2
    exit 1
  fi
fi

HEAD_COMMIT="$(git rev-parse HEAD)"
COMMIT="${RELEASE_COMMIT:-$HEAD_COMMIT}"
if [[ ! "$COMMIT" =~ ^[0-9a-f]{40}$ ]] || [ "$(git rev-parse --verify "$COMMIT^{commit}")" != "$COMMIT" ]; then
  echo "RELEASE_COMMIT must be a full commit ID available in this checkout" >&2
  exit 1
fi
if [ "$MODE" != publish ] && { [ "$COMMIT" != "$HEAD_COMMIT" ] || [ "${CI_COMMIT_SHA:-$COMMIT}" != "$COMMIT" ]; }; then
  echo "build/CI release commit must equal the checkout HEAD used by the producers" >&2
  exit 1
fi
if [ "$MODE" = build ] && [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  echo "release-atproto requires a clean source tree; commit source changes before building a pinned release" >&2
  exit 1
fi
if [ -n "${CI_COMMIT_TAG:-}" ] && [ "$(git rev-parse --verify "refs/tags/$CI_COMMIT_TAG^{commit}")" != "$COMMIT" ]; then
  echo "CI_COMMIT_TAG does not identify the release commit" >&2
  exit 1
fi

# CI already has its configured build environment. Kubernetes runners need not
# expose Docker/Podman marker files, and publishing does not need nested Docker.
if [ "$MODE" = ci ]; then
  CONTAINER=""
  run() { "$@"; }
else
  . hack/lib/container.sh >&2
fi
# Version lookup does not need any signing or publishing credentials.
PASS_ENV=(STREAMPLACE_VERSION_OVERRIDE CI_COMMIT_TAG)
VERSION="${RELEASE_VERSION:-${VERSION:-${STREAMPLACE_VERSION_OVERRIDE:-}}}"
if [ -z "$VERSION" ]; then
  if [ "$COMMIT" != "$HEAD_COMMIT" ]; then
    echo "publishing a historical commit requires an explicit VERSION or RELEASE_VERSION" >&2
    exit 1
  fi
  VERSION="$(run go run ./pkg/config/git/git.go -v)"
fi
if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  if [ "$(git rev-parse --verify "refs/tags/$VERSION^{commit}")" != "$COMMIT" ]; then
    echo "stable release version must have a real tag at the release commit" >&2
    exit 1
  fi
  CHANNEL="${RELEASE_CHANNEL:-latest}"
  if [ "$MODE" = build ]; then
    export SP_PRODUCTION_RELEASE=true
  fi
elif [[ "$VERSION" =~ ^(v[0-9]+\.[0-9]+\.[0-9]+)-([0-9a-f]{8,40})$ ]]; then
  BASE="${BASH_REMATCH[1]}"
  HASH="${BASH_REMATCH[2]}"
  if [[ "$COMMIT" != "$HASH"* ]] || ! git merge-base --is-ancestor "refs/tags/$BASE" "$COMMIT"; then
    echo "development version must use the release commit hash and an ancestor version tag" >&2
    exit 1
  fi
  CHANNEL="${RELEASE_CHANNEL:-next}"
else
  echo "release version must be vX.Y.Z or vX.Y.Z-HASH" >&2
  exit 1
fi
if [ -n "${CI_COMMIT_TAG:-}" ] && [ "$VERSION" != "$CI_COMMIT_TAG" ]; then
  echo "release version must match CI_COMMIT_TAG" >&2
  exit 1
fi
export STREAMPLACE_VERSION_OVERRIDE="$VERSION"
# A Make VERSION= override must also reach Go/Expo/Forge and recursive makes.
export VERSION

ARTIFACTS="$(realpath "${RELEASE_ARTIFACTS:-bin}")"
if [ "$MODE" = build ] && [ "$ARTIFACTS" != "$REPO/bin" ]; then
  echo "release-atproto builds into bin; use publish-atproto for an existing artifact directory" >&2
  exit 1
fi

if [ "$MODE" = build ]; then
  make all VERSION="$VERSION"
fi

# Only the publishing/collection process receives these additional variables.
PASS_ENV=(SP_RELEASE_PDS SP_RELEASE_DID SP_RELEASE_IDENTIFIER SP_RELEASE_PASSWORD)
if [ "$MODE" = ci ]; then
  PASS_ENV+=(CI_API_V4_URL CI_PROJECT_ID CI_JOB_TOKEN CI_COMMIT_TAG CI_COMMIT_BRANCH)
fi

# Existing artifacts may live outside the builder's mounted parent. Copy only
# exact-version candidates, never signing inputs, directories, or symlinks.
STAGED_INPUT=""
trap 'if [ -n "$STAGED_INPUT" ]; then rm -rf "$STAGED_INPUT"; fi' EXIT
if [ "$MODE" != ci ] && [ -n "$CONTAINER" ] && [[ "$ARTIFACTS" != "$PARENT"/* ]]; then
  mkdir -p .build
  STAGED_INPUT="$(mktemp -d "$REPO/.build/release-input.XXXXXX")"
  shopt -s nullglob
  for file in "$ARTIFACTS/streamplace-$VERSION-"* "$ARTIFACTS/streamplace-desktop-$VERSION-"*; do
    case "${file##*/}" in
      "streamplace-$VERSION-sign-input.tar.gz"|"streamplace-$VERSION-sign-input"|"streamplace-$VERSION-ios-release.xcarchive"|"streamplace-$VERSION-SHA256SUMS") continue ;;
    esac
    if [ -L "$file" ] || [ ! -f "$file" ]; then
      echo "release input must be a regular file: $file" >&2
      exit 1
    fi
    cp -p "$file" "$STAGED_INPUT/"
  done
  ARTIFACTS="$STAGED_INPUT"
fi
run node hack/release-atproto.mjs "$MODE" "$ARTIFACTS" "$VERSION" "$COMMIT" \
  "$CHANNEL" "${RELEASE_CHANGELOG:-}" "${RELEASE_DRY_RUN:-0}" "${MAC_HOST:-}"
