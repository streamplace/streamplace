#!/bin/bash

# script that gets copied into the build directory to run libstreamplace
# with the correct environment variables

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname -- "$SCRIPT_DIR")
STREAMPLACE_DEV_VERSION=$(cd -- "$REPO_ROOT" && go run ./pkg/config/git/git.go -v) \
LD_LIBRARY_PATH="$SCRIPT_DIR/lib" \
SP_DEV_FRONTEND_PROXY="http://127.0.0.1:38081" \
SP_DEV_PUBLIC_OAUTH="${SP_DEV_PUBLIC_OAUTH:-true}" \
  exec "$SCRIPT_DIR/libstreamplace" "$@"
