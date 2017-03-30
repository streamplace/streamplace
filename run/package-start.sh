#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

# Make sure we have a start script before we run it
if ! cat package.json | jq -e '.scripts.start'; then
  exit 0
fi
nodemon -w package.json -x npm run start
