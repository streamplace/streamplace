#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export NODE_PATH="$DIR/../apps"
export SK_CONFIG="${SK_CONFIG:-$DIR/../config.example.yaml}"

function bigPrint() {
  echo ""
  echo "===================================="
  echo "$1"
  echo "===================================="
  echo ""
}

export APPS_TO_BUILD="sk-utils sk-config sk-plugin-core sk-schema sk-client sk-resource twixtykit mpeg-munger sk-node sk-static sk-time pipeland bellamie gort shoko broadcast-scheduler vertex-scheduler overlay autosync"
