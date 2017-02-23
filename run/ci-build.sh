#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

export NPM_CONFIG_LOGLEVEL="warn"

npm install

npm run lint

tmp="$ROOT/.tmp/$(date +%s)"
mkdir -p $tmp

# Set up Docker push
export DOCKER_CONFIG=""
if [[ "${DOCKER_CONFIG_JSON:-}" != "" ]]; then
  echo "$DOCKER_CONFIG_JSON" | base64 --decode > "$tmp/config.json"
  export DOCKER_CONFIG="$tmp"
  # Fail fast if we're not properly docker logged in
  docker pull quay.io/streamplace/empty-image && docker push quay.io/streamplace/empty-image
fi

# This means we're authorized to build some docker images
npm whoami # so we error if that thing didn't work properly
FIX_OR_ERROR="FIX" AWS_DEFAULT_REGION="us-west-2" "$ROOT/run/full-build.sh"

rm -rf "$ROOT/tmp"
