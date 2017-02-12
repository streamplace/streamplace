#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

cd "$ROOT"
if [[ $THIS_IS_CI == "true" ]]; then
  npm install
fi

export FIX_OR_ERR=${FIX_OR_ERR:-FIX}
eslint --color --ext=jsx --ext=js .
lerna --concurrency=1 exec "$ROOT/run/package-lint.sh"
