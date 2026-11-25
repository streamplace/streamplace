#!/usr/bin/env bash
#
# Build the warm dev image: a full `make dev` of one commit, kept locally so
# that new checkouts can seed from it instead of running the cold build.
#
#   make warm-image                  # builds `next`
#   WARM_REF=origin/next make warm-image
#
# Tagged ghcr.io/streamplace/streamplace:warm-<DOCKERFILE_HASH>-<sha>. The tag
# says which toolchain and source commit it came from; hack/container.sh picks
# the newest matching image when it creates a container. Nothing is pushed.
set -euo pipefail

cd "$(dirname "$0")/.."

REF="${WARM_REF:-next}"
if ! COMMIT="$(git rev-parse --verify --quiet "${REF}^{commit}")"; then
  if ! COMMIT="$(git rev-parse --verify --quiet "origin/${REF}^{commit}")"; then
    echo "warm image: no such ref: $REF" >&2
    exit 1
  fi
fi

HASH="$(sed -n 's/^[[:space:]]*DOCKERFILE_HASH:[[:space:]]*//p' .ci/dockerfile-hash.yaml | head -1)"
if [[ -z "$HASH" ]]; then
  echo "no DOCKERFILE_HASH in .ci/dockerfile-hash.yaml" >&2
  exit 1
fi
BASE="ghcr.io/streamplace/streamplace:builder-$HASH"
SHORT="$(git rev-parse --short=12 "$COMMIT")"
TAG="ghcr.io/streamplace/streamplace:warm-$HASH-$SHORT"

# Export the commit, not the working tree, so the image is reproducible from a
# branch full of edits. The frontend build shells out to git, so give the
# context a real repository with HEAD detached at COMMIT.
CTX="$(mktemp -d)"
trap 'rm -rf "$CTX"' EXIT
echo "warm image: exporting $REF ($SHORT)"
git archive --format=tar "$COMMIT" | tar -x -C "$CTX"
GITDIR="$(git rev-parse --absolute-git-dir)"
if [[ ! -d "$GITDIR/objects" ]]; then
  echo "warm image: $GITDIR is not a repository with its own objects" >&2
  echo "(run this from a normal checkout, not a linked worktree)" >&2
  exit 1
fi
cp -a "$GITDIR" "$CTX/.git"
git -C "$CTX" config --unset core.worktree 2>/dev/null || true
if [[ "$(git -C "$CTX" rev-parse --show-toplevel)" != "$(cd "$CTX" && pwd -P)" ]]; then
  echo "warm image: refusing to check out into $CTX; it is not this repository's worktree" >&2
  exit 1
fi
git -C "$CTX" checkout --quiet --detach "$COMMIT"

# The recipe is this branch's, not the built commit's: the image is how a
# checkout builds, so it must come from the working tree even when WARM_REF
# predates the recipe.
mkdir -p "$CTX/docker"
cp docker/dev.Dockerfile "$CTX/docker/dev.Dockerfile"

echo "warm image: building $TAG (the one cold build; this takes a while)"
docker build \
  -f "$CTX/docker/dev.Dockerfile" \
  --build-arg "BUILDER_IMAGE=$BASE" \
  --build-arg "WARM_COMMIT=$COMMIT" \
  -t "$TAG" \
  "$CTX"

cat <<EOF

warm image ready: $TAG
new containers (make provision in a fresh checkout) start from it
existing containers keep their image; remove one to pick this up
EOF
