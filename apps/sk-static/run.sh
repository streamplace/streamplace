#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

for mFile in $(find /www -type f -name *.mustache); do
  echo "Compiling ${mFile}..."
  outFile="$(dirname $mFile)/$(basename $mFile .mustache)"
  # Give musta all environment variables
  node /apps/sk-config/dist/cli.js "${mFile}" > "${outFile}"
  rm "${mFile}"
done

exec nginx
