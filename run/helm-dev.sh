#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

helm init --upgrade
while ! helm list > /dev/null; do
  echo "Waiting for Tiller to be ready..."
done
npm run helm-build
helm upgrade -i --debug -f values-dev.yaml dev packages/streamplace
