#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

docker run -e FIX_OR_ERROR="ERR" -v "$ROOT":/build streamplace/sp-dev:latest /build/run/lint.sh

THIS_IS_CI="${THIS_IS_CI:-}"

# This means we're authorized to build some docker images
if [[ $THIS_IS_CI == "true" ]]; then
  apt-get update && apt-get install -y realpath jq curl
  mkdir -p ~/.docker
  echo "$DOCKER_CONFIG" | base64 --decode > ~/.docker/config.json
  echo "$NPMRC" | base64 --decode > ~/.npmrc
  "$ROOT"/run/full-build.sh
fi
