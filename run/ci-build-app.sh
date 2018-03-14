#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
SP_APP_DIR="$ROOT/packages/sp-app"

cd "$SP_APP_DIR"
npm install

if [[ "$(uname)" == "Darwin" ]]; then
  echo $CERTIFICATE_OSX_P12 | base64 --decode > /tmp/certificate.p12
  ls -alhs /tmp/certificate.p12
  security create-keychain -p mysecretpassword build.keychain
  security default-keychain -s build.keychain
  security unlock-keychain -p mysecretpassword build.keychain
  security import /tmp/certificate.p12 -P "" -k build.keychain -T /usr/bin/codesign
  security find-identity -v
  node "ci-build-app.js"
else
  docker run \
    --rm \
    -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
    -e AWS_DEFAULT_REGION="$AWS_DEFAULT_REGION" \
    -e WIN_CSC_LINK="$WIN_CSC_LINK" \
    -e WIN_CSC_KEY_PASSWORD="$WIN_CSC_KEY_PASSWORD" \
    -v "$ROOT:/streamplace" \
    -w /streamplace \
    electronuserland/electron-builder:wine \
    node /streamplace/packages/sp-app/ci-build-app.js
fi

