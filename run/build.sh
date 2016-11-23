#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$DIR/common.sh"

export NPM_CONFIG_LOGLEVEL="warn"
export NODE_PATH="$DIR/../apps"

(
  cd "$DIR/.."
  npm install
  npm run lint
)

for app in $APPS_TO_BUILD; do
  (
    bigPrint "Building $app"
    cd "$DIR/../apps/$app"
    make
  )
done
