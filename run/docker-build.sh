#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$DIR/common.sh"

docker run \
  -v "$DIR/..":/streamkitchen \
  -v /run/docker.sock:/run/docker.sock \
  streamkitchen/sk-builder \
  /streamkitchen/run/build.sh
