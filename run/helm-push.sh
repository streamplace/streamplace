#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

if [[ "${AWS_ACCESS_KEY_ID:-}" == "" ]]; then
  echo "No AWS_ACCESS_KEY_ID found, not pushing charts."
  exit 0
fi

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
aws s3 --region us-west-2 sync . s3://charts.stream.place
