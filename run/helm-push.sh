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
docker run \
  --rm \
  -v "$(pwd)":"$(pwd)" \
  -w "$(pwd)" \
  -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
  -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
  -e AWS_DEFAULT_REGION="us-west-2" \
  cgswong/aws:aws -- \
  s3 sync . s3://charts.stream.place
