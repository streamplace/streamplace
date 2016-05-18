#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$DIR/common.sh"

for app in $APPS_TO_BUILD; do
  (
    bigPrint "Pushing $app"
    cd "$DIR/../apps/$app"
    make push
  )
done
