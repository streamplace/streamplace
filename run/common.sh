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

export APPS_TO_BUILD="twixtykit corporate-bullshit mpeg-munger sk-node sk-schema sk-static sk-time sk-client sk-ffmpeg pipeland bellamie gort shoko"
