#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

docker run --rm -e FIX_OR_ERROR="ERR" -v "$DIR":/build streamplace/sp-dev:latest /build/run/ci-build-container.sh
