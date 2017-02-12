#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

docker run \
  -e FIX_OR_ERROR="ERR" \
  -e NPM_CONFIG_LOGLEVEL="warn" \
  -e THIS_IS_CI="$THIS_IS_CI" \
  -v "$ROOT":/build streamplace/sp-dev:latest /build/run/lint.sh

# This means we're authorized to build some docker images
if [[ $THIS_IS_CI == "true" ]]; then
  mkdir -p ~/.docker
  echo "$DOCKER_CONFIG_JSON" | base64 --decode > ~/.docker/config.json
  echo "$NPMRC" | base64 --decode > ~/.npmrc
  docker run \
    -e FIX_OR_ERROR="FIX" \
    -e NPM_CONFIG_LOGLEVEL="warn" \
    -v ~/.docker/config.json:/root/.docker/config.json \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ~/.npmrc:/root/.npmrc \
    -v "$ROOT":/build streamplace/sp-dev:latest /build/run/full-build.sh
fi
