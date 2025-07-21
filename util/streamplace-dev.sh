#!/bin/bash

# script that gets copied into the build directory to run libstreamplace
# with the correct environment variables

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
STREAMPLACE_DEV_VERSION=$(go run ./pkg/config/git/git.go -v) \
LD_LIBRARY_PATH="$SCRIPT_DIR/lib/usr/local/lib/x86_64-linux-gnu" \
DYLD_LIBRARY_PATH="$SCRIPT_DIR/lib/usr/local/lib" \
SP_DEV_FRONTEND_PROXY="http://127.0.0.1:38081" \
  exec "$SCRIPT_DIR/libstreamplace" "$@"
