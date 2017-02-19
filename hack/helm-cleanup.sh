#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

resources="$(find $ROOT/packages/*/templates/*.yaml | xargs basename -s .yaml | sort | uniq)"
for resource in $resources; do
  echo "deleting $resource"
  deleteThese="$(kubectl get -o name $resource | grep -v kubernetes || echo '')"
  if [[ -n "$deleteThese" ]]; then
    echo "$deleteThese" | xargs -L 1 kubectl delete
  fi
done
