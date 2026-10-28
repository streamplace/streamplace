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

# make version writes branch metadata too, even for a local-only build.
if [ -z "${CI_COMMIT_TAG:-}${CI_COMMIT_BRANCH:-}" ]; then
  export CI_COMMIT_BRANCH="$(git -C "$REPO" symbolic-ref --quiet --short HEAD || printf local)"
fi

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
  STREAMPLACE_VERSION_OVERRIDE
  SP_PRODUCTION_RELEASE
  CI_COMMIT_TAG
  CI_COMMIT_BRANCH
  STREAMPLACE_RUN_DIR
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
    local key_path=""
    if [[ " ${PASS_ENV[*]} " == *" APPLE_CODESIGN_API_KEY "* ]]; then
      key_path="${APPLE_CODESIGN_API_KEY:-}"
    fi
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
        # Docker accepts -e NAME, reading the value from its own environment.
        # Keep credentials out of the docker exec argument list.
        env_args+=(-e "$name")
      fi
    done

    local run_args=()
    if [ -n "${STREAMPLACE_RUN_DIR:-}" ]; then
      # Run the command in its own session and record the session leader PID so
      # the caller can stop exactly this run's process tree.
      run_args=(bash -c 'setsid --wait bash -c '\''echo $$ > "$STREAMPLACE_RUN_DIR/pid.$$"; exec "$@"'\'' bash "$@"' bash "$@")
    else
      run_args=("$@")
    fi

    if [ -n "$key_dir" ]; then
      ( trap 'rm -rf "$key_dir"' EXIT; APPLE_CODESIGN_API_KEY="$key_path" docker exec -w "$REPO" "${env_args[@]}" "$CONTAINER" "${run_args[@]}" )
    else
      APPLE_CODESIGN_API_KEY="$key_path" docker exec -w "$REPO" "${env_args[@]}" "$CONTAINER" "${run_args[@]}"
    fi
  fi
}
