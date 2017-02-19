#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

cd "$ROOT"
npm install
mkdir -p "$ROOT/build_chart"
rm -rf "$ROOT/build_chart/*"
cd "$ROOT/build_chart"
mv "$ROOT"/packages/*/*.tgz .
oldIndex=$(mktemp).yaml
curl -o $oldIndex "https://charts.stream.place/index.yaml"
helm repo index --url https://charts.stream.place --merge $oldIndex .
rm $oldIndex
aws --region us-west-2 s3 sync . s3://charts.stream.place
