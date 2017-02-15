#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if [[ ! -f Dockerfile ]]; then
  exit 0
fi

beforeContainer="streamplace/$PACKAGE_NAME:latest"

info "Building container for $PACKAGE_NAME"
docker build -t streamplace/$PACKAGE_NAME:latest .
