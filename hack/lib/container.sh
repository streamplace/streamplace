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

# `docker exec` starts a new process with the container's environment, not the
# host shell's. Forward credentials and signing controls explicitly so a host
# `make all` signs with the identity the caller exported instead of silently
# producing ad-hoc artifacts. Kept to an allowlist: don't leak unrelated host
# variables into the builder.
PASS_ENV=(
  DEVELOPER_CERTIFICATE_BASE64
  DEVELOPER_CERTIFICATE_PASSWORD
  APPLE_CODESIGN_API_KEY
  SIGN_NOTARIZE
  SIGN_TIMESTAMP_URL
  SIGN_OFFLINE
  SIGN_NOTARY_WAIT
  ALLOW_ADHOC
  ANDROID_KEYSTORE_BASE64
  ANDROID_KEYSTORE_PASSWORD
  ANDROID_KEYSTORE_ALIAS
)

run() {
  if [ -z "$CONTAINER" ]; then
    "$@"
  else
    local env_args=()
    local name
    for name in "${PASS_ENV[@]}"; do
      if [ -n "${!name+x}" ]; then
        env_args+=(-e "$name=${!name}")
      fi
    done
    docker exec -w "$REPO" "${env_args[@]}" "$CONTAINER" "$@"
  fi
}
