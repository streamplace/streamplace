#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

docker run -it --rm --net host -v $(pwd)/entrypoint.sh:/entrypoint.sh --privileged kurento/kurento-media-server:6.6.0
