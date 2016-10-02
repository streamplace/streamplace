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

export APPS_TO_BUILD="sk-utils sk-config sk-schema sk-client sk-resource sk-plugin-core twixtykit mpeg-munger sk-node sk-static sk-time pipeland bellamie gort shoko broadcast-scheduler vertex-scheduler overlay autosync"
