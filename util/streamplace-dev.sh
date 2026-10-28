#!/bin/bash

# script that gets copied into the build directory to run libstreamplace
# with the correct environment variables

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# The Makefile copies this script into $(BUILDDIR), which can be nested
# (e.g. out/dev); find the checkout by its go.mod instead of assuming the
# build directory's parent is the repo root.
REPO_ROOT="$SCRIPT_DIR"
while [ ! -f "$REPO_ROOT/go.mod" ] && [ "$REPO_ROOT" != "/" ]; do
  REPO_ROOT=$(dirname -- "$REPO_ROOT")
done
if [ ! -f "$REPO_ROOT/go.mod" ] || [ ! -f "$REPO_ROOT/pkg/config/git/git.go" ]; then
  echo "cannot locate the streamplace checkout from $SCRIPT_DIR" >&2
  exit 1
fi
STREAMPLACE_DEV_VERSION=$(cd -- "$REPO_ROOT" && go run ./pkg/config/git/git.go -v) \
LD_LIBRARY_PATH="$SCRIPT_DIR/lib" \
SP_DEV_FRONTEND_PROXY="http://127.0.0.1:38081" \
SP_DEV_PUBLIC_OAUTH="${SP_DEV_PUBLIC_OAUTH:-true}" \
  exec "$SCRIPT_DIR/libstreamplace" "$@"
