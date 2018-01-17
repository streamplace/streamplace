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

domain="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.domain')"
ip="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.externalIP')"
hostsLine="$ip $domain"
if ! cat /etc/hosts | grep "$hostsLine" > /dev/null; then
  info "Need to update /etc/hosts"
  info "Please give me sudo powers to run this command:"
  cmd="$ROOT/node_modules/.bin/hostile set $ip $domain"
  echo "$cmd"
  sudo bash -c "$cmd"
fi

# hack hack hack
export WH_EXTERNAL_IP="$(js-yaml $ROOT/values-dev.yaml | jq -r '.global.externalIP')"

wheelhouse link
wheelhouse build
npm run helm-dev

nodemon -w 'packages/**/Dockerfile' --on-change-only -x wheelhouse build docker &
run/every-package.sh run/package-start.sh --no-sort --concurrency 999
wait
