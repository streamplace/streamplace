#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

export NPM_CONFIG_LOGLEVEL="warn"

apt-get update && apt-get install -y realpath jq curl awscli

npm config set unsafe-perm true
npm config set '//registry.npmjs.org/:_authToken' $NPM_TOKEN

npm install --unsafe-perm

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

export AWS_DEFAULT_REGION="us-west-2"
# This means we're authorized to build some docker images
npm whoami # so we error if that thing didn't work properly
CI_TRIGGER_APP_BUILDS="true" FIX_OR_ERROR="FIX" "$ROOT/run/full-build.sh"

rm -rf "$ROOT/tmp"

message="Version *v$REPO_VERSION* (branch $REPO_BRANCH) is cooked and ready to go. ❤️"
curl -X POST -H 'Content-type: application/json' --data "{\"text\":\"$message\"}" "$SLACK_URL"
