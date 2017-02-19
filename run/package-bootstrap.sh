#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if [[ ! -d node_modules ]]; then
  exit 0
fi

links=$(find node_modules -type l -maxdepth 1)

if [[ "$links" != "" ]]; then
  echo "Replacing Lerna's relative symlinks with absolute ones..."
fi

for linkPath in $links; do
  actualPath="$(realpath "$linkPath")"
  rm "$linkPath"
  ln -s "$actualPath" "$linkPath"
done
