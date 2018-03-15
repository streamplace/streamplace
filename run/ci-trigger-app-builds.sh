#!/bin/bash

# Triggers AppVeyor and Travis builds of the Electron app after CircleCI completes.

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

BRANCH="$1"
TRAVIS_TOKEN="$(cat /keybase/team/streamplace_team/secrets/travis_token)"

body="$(jq -n ".request.branch = \"$BRANCH\"")"

curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Travis-API-Version: 3" \
  -H "Authorization: token $TRAVIS_TOKEN" \
  -d "$body" \
  https://api.travis-ci.org/repo/streamplace%2Fstreamplace/requests
