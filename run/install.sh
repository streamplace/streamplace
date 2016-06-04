#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$DIR/common.sh"

# Only non-node dependency here is libzmq for pipeland. We want a particular version, so let's
# just install it locally.

(
  cd "$LIBDIR"
  PREFIX="$LIBDIR" "$DIR/../apps/pipeland/get-zmq.sh"
)
