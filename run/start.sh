#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

export LOCAL_DEV="true"

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

# Clean up all subprocesses on exit, we have many
trap "exit" INT TERM
trap "kill 0" EXIT

npm run build-values-dev
npm run kube-init
npm run update-cert
wheelhouse link
wheelhouse build

nodemon -w 'packages/**/Dockerfile' --on-change-only -x wheelhouse build docker &
run/every-package.sh run/package-start.sh --no-sort --concurrency 999
wait
