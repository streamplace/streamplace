#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

script="$1"
shift
# If we were passed a shell script, make sure it's the absolute path
if [[ -f "$script" ]]; then
  script="$(realpath $script)"
fi
cmd="lerna exec $(realpath "$ROOT/run/package-log.sh") $script $*"
echo "running $cmd"
$cmd
