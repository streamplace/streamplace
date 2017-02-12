#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

docker run -e FIX_OR_ERROR="ERR" -v "$DIR":/build streamplace/sp-dev:latest /build/run/lint.sh

THIS_IS_CI="${THIS_IS_CI:-}"

# This means we're authorized to build some docker images
if [[ $THIS_IS_CI == "true" ]]; then
  apt-get update && apt-get install -y realpath jq curl
  mkdir -p ~/.docker
  echo "$DOCKER_CONFIG" | base64 --decode > ~/.docker/config.json
  echo "$NPMRC" | base64 --decode > ~/.npmrc
  docker run \
    -e FIX_OR_ERROR="FIX" \
    -v ~/.docker/config.json:/root/.docker/config.json \
    -v ~/.npmrc:/root/.npmrc \
    -v "$DIR":/build streamplace/sp-dev:latest /build/run/full-build.sh
fi
