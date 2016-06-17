#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

DOCKER_MACHINE_NAME="${DOCKER_MACHINE_NAME:-}"

if [[ "$DOCKER_MACHINE_NAME" != "" ]]; then
  echo "DOCKER_MACHINE_NAME detected. Attempting to use docker-machine to forward relevant ports..."
  docker-machine ssh "$DOCKER_MACHINE_NAME" -T \
    -L 5000:localhost:5000 \
    -L 1935:localhost:1935 \
    -L 8100:localhost:8100 tail -f /dev/null &
fi

cd "$DIR/.."
docker-compose up
