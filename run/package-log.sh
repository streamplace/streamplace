#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

prettylog() {
  name="$PACKAGE_NAME"
  color="$(node "$ROOT/run/get-color.js" $name)"
  while IFS= read -r line; do
    printf "\e[38;05;${color}m${name}\e[38;05;231m %s\n" "$line"
  done
}

$* 2>&1 | prettylog
