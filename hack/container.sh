#!/usr/bin/env bash
#
# Make sure this checkout has its long-running build container, and optionally
# build the dev environment inside it. Idempotent — safe to re-run, and it
# adopts a container someone else already made for this checkout.
#
#   hack/container.sh ensure      # container exists and is running
#   hack/container.sh dev-setup   # ensure, then `make dev-setup` inside it
#   hack/container.sh path        # ensure, then print the build path inside it
#   hack/container.sh name        # print the running container's name
#
# The container is named after the checkout directory (streamplace-N), mounts
# the checkout's parent at the same absolute path (so sibling checkouts are
# visible), and also mounts the checkout at /streamplace, where new builds and
# the warm image expect it. It comes from the newest local warm image
# (hack/warm-image.sh) if there is one, otherwise from the builder image pinned
# in .ci/dockerfile-hash.yaml, and its first start seeds the checkout from the
# warm image's baked build. See .claude/skills/streamplace-docker/SKILL.md.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
NAME="$(basename "$REPO")"
PARENT="$(dirname "$REPO")"
MODE="${1:-ensure}"

case "$MODE" in
  ensure|dev-setup|path|name) ;;
  *) echo "usage: hack/container.sh [ensure|dev-setup|path|name]" >&2; exit 2 ;;
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
IMAGE="ghcr.io/streamplace/streamplace:builder-$HASH"
WARM_REPO="ghcr.io/streamplace/streamplace"

# The newest local warm image for this builder hash, if any. It is the same
# toolchain with a full `make dev` of one commit baked in (hack/warm-image.sh).
# STREAMPLACE_WARM_IMAGE overrides the choice; STREAMPLACE_WARM=0 opts out.
warm_image() {
  case "${STREAMPLACE_WARM:-1}" in
    0 | no | false) return 1 ;;
  esac
  if [[ -n "${STREAMPLACE_WARM_IMAGE:-}" ]]; then
    printf '%s\n' "$STREAMPLACE_WARM_IMAGE"
    return 0
  fi
  local tag
  tag="$(docker images --format '{{.CreatedAt}}\t{{.Repository}}:{{.Tag}}' \
    --filter "reference=$WARM_REPO:warm-$HASH-*" 2>/dev/null \
    | sort -r | head -1 | cut -f2)"
  [[ -n "$tag" ]] || return 1
  printf '%s\n' "$tag"
}

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
  RUN_IMAGE="$IMAGE"
  if WARM="$(warm_image)"; then
    RUN_IMAGE="$WARM"
  fi
  docker run -d \
    -w /streamplace \
    -v "$PARENT:$PARENT" \
    -v "$REPO:/streamplace" \
    -e LD_LIBRARY_PATH=/streamplace/build-linux-amd64/lib \
    -e PKG_CONFIG_PATH=/streamplace/build-linux-amd64/lib/pkgconfig \
    --name "$NAME" \
    "$RUN_IMAGE" \
    tail -f /dev/null >/dev/null
  CONTAINER="$NAME"
  echo "created container $NAME from $RUN_IMAGE"
  if [[ "$RUN_IMAGE" == "$IMAGE" ]]; then
    echo "note: no local warm image for builder-$HASH, so this checkout cold-builds."
    echo "      run 'make warm-image' once to give later checkouts a warm start."
  fi
fi

ACTUAL="$(docker inspect --format '{{.Config.Image}}' "$CONTAINER")"
if [[ "$ACTUAL" != "$IMAGE" && "$ACTUAL" != *:warm-* ]]; then
  echo "note: $CONTAINER runs $ACTUAL, but .ci/dockerfile-hash.yaml now says $IMAGE"
fi
echo "$CONTAINER is up"

# Where the checkout lives for build purposes. Containers created here mount it
# at /streamplace; older containers only have the host path.
WORKDIR="$REPO"
if docker exec "$CONTAINER" test -d /streamplace 2>/dev/null; then
  WORKDIR=/streamplace
  docker exec -w /streamplace "$CONTAINER" bash hack/warm-seed.sh
elif WARM="$(warm_image)"; then
  echo "note: $CONTAINER predates the /streamplace mount, so it cannot seed from"
  echo "      $WARM. Remove and re-create it to use the warm image."
fi

if [[ "$MODE" == path ]]; then
  printf '%s\n' "$WORKDIR"
  exit 0
fi

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
  exec docker exec -w "$WORKDIR" "$CONTAINER" make dev-setup
fi
