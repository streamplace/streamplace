#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if [[ ! -f Chart.yaml ]]; then
  exit 0
fi

cd "$ROOT/build_chart"
helm package "$ROOT/packages/$PACKAGE_NAME"
