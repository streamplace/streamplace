#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

function bigPrint() {
  echo ""
  echo "===================================="
  echo "$1"
  echo "===================================="
  echo ""
}

LIBDIR="$DIR/../.lib";
mkdir -p "$LIBDIR"
export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:-} $LIBDIR"

export APPS_TO_BUILD="twixtykit mpeg-munger sk-node sk-schema sk-static sk-time sk-client pipeland bellamie gort shoko"
