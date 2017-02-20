#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

export LOCAL_DEV="true"

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

nodemon -w 'packages/**/Dockerfile' -x npm run docker-build &
run/every-package.sh run/package-start.sh --no-sort --concurrency 999
wait
