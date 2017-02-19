#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if [[ ! -f Chart.yaml ]]; then
  exit 0
fi

if [[ -d charts ]]; then
  rm -rf charts/*
fi

if [[ -f requirements.yaml ]]; then
  mkdir -p charts
  requirementsJson="$(js-yaml requirements.yaml)"
  requirements="$(echo $requirementsJson | jq -r '.dependencies[].name')"
  for req in $requirements; do
    echo "$req"
    cp -v "$ROOT"/packages/$req/*.tgz charts
  done
fi

helm package --debug .
