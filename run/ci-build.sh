#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

npm install

docker run \
  -e FIX_OR_ERROR="ERR" \
  -e NPM_CONFIG_LOGLEVEL="warn" \
  -e THIS_IS_CI="$THIS_IS_CI" \
  -v "$ROOT":/build streamplace/sp-dev:latest /build/run/lint.sh

tmp="$ROOT/tmp/$(date +%s)"
mkdir -p $tmp

# This means we're authorized to build some docker images
if [[ $THIS_IS_CI == "true" ]]; then
  echo "$DOCKER_CONFIG_JSON" | base64 --decode > "$tmp/config.json"
  echo "$NPMRC" | base64 --decode > "$tmp/npmrc"
  docker run \
    -e FIX_OR_ERROR="FIX" \
    -e NPM_CONFIG_LOGLEVEL="warn" \
    -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
    -e AWS_DEFAULT_REGION="us-west-2" \
    -v "$tmp/config.json":/root/.docker/config.json \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$tmp/npmrc":/root/.npmrc \
    -v "$ROOT":/build streamplace/sp-dev:latest /build/run/full-build.sh
fi

rm -rf "$ROOT/tmp"
