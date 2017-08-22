#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

helm init --upgrade
while ! helm list > /dev/null; do
  echo "Waiting for Tiller to be ready..."
  sleep 1
done
wheelhouse build helm
helm upgrade -i -f values-dev.yaml dev packages/streamplace
