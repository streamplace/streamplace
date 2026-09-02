#!/bin/bash

# script that gets copied into the build directory to run libstreamplace
# with the new Vite frontend (proxied to Vite dev server on :5173)

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# macOS uses DYLD_LIBRARY_PATH; Linux uses LD_LIBRARY_PATH.
# Set both so the script works regardless of platform.
LD_LIBRARY_PATH="$SCRIPT_DIR/lib" \
DYLD_LIBRARY_PATH="$SCRIPT_DIR/lib" \
SP_DEV_FRONTEND_PROXY="http://127.0.0.1:5173" \
SP_DEV_PUBLIC_OAUTH=true \
  exec "$SCRIPT_DIR/libstreamplace" "$@"
