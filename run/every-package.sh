#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

script="$1"
shift
cmd="lerna exec $* $(realpath "$ROOT/run/package-log.sh") $(realpath $script)"
echo "running $cmd"
$cmd
