#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export SK_CONFIG="$DIR/../config.yml";

prettylog() {
  name="$1"
  color="$2"
  while IFS= read -r line; do
    echo -en "\e[38;05;${color}m[${name}]\e[38;05;231m "
    echo "$line"
  done
}

function run() {
  name="$1"
  path="$name"
  color="$2"
  cd "$DIR/../apps/$path" && npm run dev 2>&1 | prettylog "$name" "$color" &
  sleep 1
}
export NODE_PATH="$NODE_PATH:$(realpath "$DIR/../apps")"
# export DEBUG_LEVEL="debug"
run sk-config 1
run sk-schema 4
# run sk-code 190
run shoko 208
run sk-client 196
run mpeg-munger 214
run sk-time 94
run bellamie 201
run gort 6
run pipeland 40
run vertex-scheduler 50
run broadcast-scheduler 60
run autosync 70
run overlay 80
wait

# for i in {0..255}; do echo -e "\e[38;05;${i}m\\\e[38;05;${i}m"; done
