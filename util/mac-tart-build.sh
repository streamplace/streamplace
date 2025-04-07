#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
DIR="$(realpath $SCRIPT_DIR/..)"

tart list --format json | jq -r '.[] | select(.State == "running") | .Name' | xargs -L 1 tart stop
IMAGE=sonoma-$(date +%s)
tart clone ghcr.io/cirruslabs/macos-sonoma-xcode:15.3 $IMAGE
bash -c "tart run $IMAGE --no-graphics --dir=streamplace:$DIR --dir=signing:/Volumes/UnlockedKey &"
while ! tart ip $IMAGE; do echo 'waiting for ip...' && sleep 1; done;
export EXIT="0"
cat util/mac-build.sh | sshpass -p admin ssh -o "StrictHostKeyChecking no" admin@$(tart ip $IMAGE) bash -c 'cat > mac-build.sh && bash mac-build.sh' || export EXIT=1
# tart stop $IMAGE
# if [[ $EXIT != "0" ]]; then
#   echo "build failed"
#   exit $EXIT
# fi
