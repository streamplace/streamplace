#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

npm install
lerna bootstrap
run/every-package.sh run/package-bootstrap.sh --concurrency=999 --no-sort
