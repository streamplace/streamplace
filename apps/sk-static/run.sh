#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

for mFile in $(find webs -type f -name *.mustache); do
  echo "Compiling ${mFile}..."
  outFile="$(dirname $mFile)/$(basename $mFile .mustache)"
  # Give musta all environment variables
  musta -t "${mFile}" $(printenv) > "${outFile}"
  rm "${mFile}"
done

exec nginx
