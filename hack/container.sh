#!/usr/bin/env bash
#
# Make sure this checkout has its long-running build container, and optionally
# build the dev environment inside it. Idempotent — safe to re-run, and it
# adopts a container someone else already made for this checkout.
#
#   hack/container.sh ensure      # container exists and is running
#   hack/container.sh dev-setup   # ensure, then `make dev-setup` inside it
#
# The container is named after the checkout directory (streamplace-N), mounts
# the checkout's parent at the same absolute path (so sibling checkouts are
# visible), and comes from the builder image pinned in
# .ci/dockerfile-hash.yaml. See AGENTS.md §1 and §2.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
NAME="$(basename "$REPO")"
PARENT="$(dirname "$REPO")"
MODE="${1:-ensure}"

case "$MODE" in
  ensure|dev-setup|name) ;;
  *) echo "usage: hack/container.sh [ensure|dev-setup|name]" >&2; exit 2 ;;
esac

# `name` prints the running container for this checkout and nothing else, for
# callers that need to docker exec into it themselves.
if [[ "$MODE" == name ]]; then
  for candidate in "$NAME" "$NAME-builder"; do
    if docker ps --format '{{.Names}}' | grep -qx "$candidate"; then
      echo "$candidate"
      exit 0
    fi
  done
  echo "no running container for $NAME — run 'make container' first" >&2
  exit 1
fi

if [[ "$NAME" != streamplace* ]]; then
  echo "refusing: $REPO is not a streamplace-* checkout, so the container would" >&2
  echo "be named '$NAME'. Run this from the checkout you were assigned." >&2
  exit 1
fi

# Already inside a container: there is nothing to ensure, just do the work.
# Both markers are checked because these containers are not always created by
# dockerd itself (/.dockerenv is absent under podman; /run/.containerenv is
# absent under docker).
if [[ -f /.dockerenv || -f /run/.containerenv ]]; then
  echo "already inside a container; running in place"
  if [[ "$MODE" == dev-setup ]]; then exec make dev-setup; fi
  exit 0
fi

if ! command -v docker >/dev/null; then
  echo "docker not found on PATH, and this does not look like a container" >&2
  echo "(no /.dockerenv or /run/.containerenv). Run this from the host." >&2
  exit 1
fi

HASH="$(sed -n 's/^[[:space:]]*DOCKERFILE_HASH:[[:space:]]*//p' .ci/dockerfile-hash.yaml | head -1)"
if [[ -z "$HASH" ]]; then
  echo "no DOCKERFILE_HASH in .ci/dockerfile-hash.yaml" >&2
  exit 1
fi
IMAGE="public.ecr.aws/m4j3c0j7/streamplace:builder-$HASH"

running() { docker ps --format '{{.Names}}' | grep -qx "$1"; }
exists() { docker ps -a --format '{{.Names}}' | grep -qx "$1"; }

# `docker ps` hides stopped containers, and the container may also be named
# streamplace-N-builder, so check all four cases before creating anything.
if running "$NAME"; then
  CONTAINER="$NAME"
elif running "$NAME-builder"; then
  CONTAINER="$NAME-builder"
elif exists "$NAME"; then
  docker start "$NAME" >/dev/null
  CONTAINER="$NAME"
  echo "started stopped container $NAME"
elif exists "$NAME-builder"; then
  docker start "$NAME-builder" >/dev/null
  CONTAINER="$NAME-builder"
  echo "started stopped container $NAME-builder"
else
  docker run -d \
    -w "$REPO" \
    -v "$PARENT:$PARENT" \
    -e LD_LIBRARY_PATH="$REPO/build-linux-amd64/lib" \
    -e PKG_CONFIG_PATH="$REPO/build-linux-amd64/lib/pkgconfig" \
    --name "$NAME" \
    "$IMAGE" \
    tail -f /dev/null >/dev/null
  CONTAINER="$NAME"
  echo "created container $NAME from $IMAGE"
fi

ACTUAL="$(docker inspect --format '{{.Config.Image}}' "$CONTAINER")"
if [[ "$ACTUAL" != "$IMAGE" ]]; then
  echo "note: $CONTAINER runs $ACTUAL, but .ci/dockerfile-hash.yaml now says $IMAGE"
fi
echo "$CONTAINER is up"

if [[ "$MODE" == dev-setup ]]; then
  # `make dev-setup` is single-shot: it calls `meson setup`, which refuses to
  # touch an already-configured directory ("Directory already configured",
  # exit 1). Mirror the guard `make dev` uses instead of failing on re-run.
  if [[ -d "$REPO/build-linux-amd64" ]]; then
    echo "build-linux-amd64 already exists; skipping make dev-setup"
    echo "(rm -rf build-linux-amd64 to build it from scratch)"
    exit 0
  fi
  echo "running make dev-setup in $CONTAINER"
  exec docker exec -w "$REPO" "$CONTAINER" make dev-setup
fi
