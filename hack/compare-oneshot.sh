#!/bin/bash

# Test that we can combine then split segments and get the same files

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Detect platform/arch for correct build dir
UNAME_S=$(uname -s | tr '[:upper:]' '[:lower:]')
UNAME_M=$(uname -m)
if [[ "$UNAME_S" == "darwin" && "$UNAME_M" == "arm64" ]]; then
  BUILD_DIR="build-darwin-arm64"
elif [[ "$UNAME_S" == "linux" && "$UNAME_M" == "x86_64" ]]; then
  BUILD_DIR="build-linux-amd64"
else
  echo "Unsupported platform: $UNAME_S/$UNAME_M"
  exit 1
fi

DEBUG_DIR="$(mktemp -d)"
mkdir -p "$DEBUG_DIR/segments"
set +e
$SCRIPT_DIR/../$BUILD_DIR/streamplace combine --debug-dir="$DEBUG_DIR/segments-1" "$DEBUG_DIR/combined.mp4" $(find "$@" -name '*.mp4' | sort)
EXIT_CODE=$?
set -e
if [ $EXIT_CODE -ne 0 ]; then
  $SCRIPT_DIR/compare-hash.sh "$@" "$DEBUG_DIR/segments-1"
  exit $EXIT_CODE
fi
echo "Success"
