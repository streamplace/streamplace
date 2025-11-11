#!/bin/bash

# Test that we can combine then split segments and get the same files

set -euo pipefail
set -x

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

DEBUG_DIR="$(mktemp -d)"
mkdir -p "$DEBUG_DIR/segments"
set +e
$SCRIPT_DIR/../build-darwin-arm64/streamplace combine --debug-dir="$DEBUG_DIR/segments-1" "$DEBUG_DIR/combined.mp4" $(find "$@" -name '*.mp4' | sort)
$SCRIPT_DIR/../build-darwin-arm64/streamplace combine --debug-dir="$DEBUG_DIR/segments-2" "$DEBUG_DIR/combined2.mp4" $(find "$DEBUG_DIR/segments-1" -name '*.mp4' | sort)
EXIT_CODE=$?
set -e
if [ $EXIT_CODE -ne 0 ]; then
  $SCRIPT_DIR/compare-hash.sh "$DEBUG_DIR/segments-1" "$DEBUG_DIR/segments-2"
  exit $EXIT_CODE
fi
echo "Success"
