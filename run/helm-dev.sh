#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

helm init --upgrade
kubectl apply -f "$ROOT/hack/local-logger.yaml"
while ! helm list > /dev/null; do
  echo "Waiting for Tiller to be ready..."
  sleep 1
done
wheelhouse build helm
touch values-dev.yaml
helm upgrade -i -f values-dev.yaml -f values.local.yaml --set global.rootDirectory="$ROOT" dev packages/streamplace
