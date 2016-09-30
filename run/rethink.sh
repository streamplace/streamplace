#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

datadir="$(realpath "$DIR/../rethinkdb_data")"
mkdir -p "$datadir"
docker run -d -v "$datadir":/data/rethinkdb_data --name rethink -p 18080:8080 -p 28015:28015 -p 29015:29015 rethinkdb:2.3
