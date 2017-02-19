#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if [[ ! -f Dockerfile ]]; then
  exit 0
fi

beforeContainer="streamplace/$PACKAGE_NAME:latest"
taggedContainer="$DOCKER_PREFIX/$PACKAGE_NAME:v$REPO_VERSION"

# Tag later, so the streamplace/whatever tag can be used to build other images.
info "Tagging $beforeContainer as $taggedContainer"
docker tag $beforeContainer $taggedContainer
docker push $taggedContainer
