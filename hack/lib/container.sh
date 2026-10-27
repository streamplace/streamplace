# shellcheck shell=bash
# Source from a repo-root script to run commands in the checkout's pinned
# builder container. Sets REPO, NAME, and CONTAINER and defines run().
#
# The caller is expected to have `set -euo pipefail` and to have changed to the
# repo root; REPO may be preset.
REPO="${REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
PARENT="$(dirname "$REPO")"
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
  CI_COMMIT_TAG
  CI_COMMIT_BRANCH
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
    # The container only mounts the checkout's parent. If the Apple key lives
    # elsewhere, copy it into that mount and forward the container-visible
    # path; rcodesign opens the path itself and cannot see the host's home.
    local key_dir=""
    local key_path="${APPLE_CODESIGN_API_KEY:-}"
    if [ -n "$key_path" ]; then
      case "$key_path" in
        /*) ;;
        *) key_path="$REPO/$key_path" ;;
      esac
      if [ ! -f "$key_path" ]; then
        echo "APPLE_CODESIGN_API_KEY does not exist: $key_path" >&2
        return 1
      fi
      if [[ "$key_path" != "$PARENT"/* ]]; then
        key_dir="$(mktemp -d "$PARENT/.streamplace-key.XXXXXX")"
        cp "$key_path" "$key_dir/key.json"
        chmod 700 "$key_dir"
        chmod 600 "$key_dir/key.json"
        key_path="$key_dir/key.json"
      fi
    fi

    local env_args=()
    local name
    for name in "${PASS_ENV[@]}"; do
      if [ -n "${!name+x}" ]; then
        local value="${!name}"
        if [ "$name" = "APPLE_CODESIGN_API_KEY" ]; then
          value="$key_path"
        fi
        env_args+=(-e "$name=$value")
      fi
    done

    if [ -n "$key_dir" ]; then
      ( trap 'rm -rf "$key_dir"' EXIT; docker exec -w "$REPO" "${env_args[@]}" "$CONTAINER" "$@" )
    else
      docker exec -w "$REPO" "${env_args[@]}" "$CONTAINER" "$@"
    fi
  fi
}
