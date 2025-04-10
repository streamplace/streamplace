#!/bin/bash

# script that gets copied into the build directory to run libstreamplace
# with the correct environment variables

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
LD_LIBRARY_PATH="$SCRIPT_DIR/lib/usr/local/lib/x86_64-linux-gnu" DYLD_LIBRARY_PATH="$SCRIPT_DIR/lib/usr/local/lib" exec "$SCRIPT_DIR/libstreamplace" "$@"
