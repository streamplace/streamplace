#!/bin/bash

set -o pipefail
set -o nounset
set -o errexit

BRANCH="$1"
COMMIT="$2"
SECRETS="/keybase/team/streamplace_team/secrets"

BODY="$(cat << EOF
{
  "build_parameters": {}
}
EOF
)"

curl \
  -X POST \
  --header "Content-Type: application/json" \
  -d "$BODY" \
  "https://circleci.com/api/v1.1/project/github/streamplace/streamplace/tree/$BRANCH?circle-token=$CIRCLECI_TOKEN"
