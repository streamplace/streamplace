# shellcheck shell=bash
# Source from a repo-root script to run commands in the checkout's pinned
# builder container. Sets REPO, NAME, and CONTAINER and defines run().
#
# The caller is expected to have `set -euo pipefail` and to have changed to the
# repo root; REPO may be preset.
REPO="${REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
NAME="$(basename "$REPO")"
CONTAINER=""

if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
  echo "already inside a container; running in place"
else
  if [[ "$NAME" != streamplace* ]]; then
    echo "refusing: $REPO is not a streamplace-* checkout" >&2
    return 1
  fi
  bash "$REPO/hack/container.sh" ensure
  CONTAINER="$(bash "$REPO/hack/container.sh" name)"
fi

run() {
  if [ -z "$CONTAINER" ]; then
    "$@"
  else
    docker exec -w "$REPO" "$CONTAINER" "$@"
  fi
}
