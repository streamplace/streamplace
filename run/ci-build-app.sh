#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

echo $CERTIFICATE_OSX_P12 | base64 --decode > /tmp/certificate.p12
ls -alhs /tmp/certificate.p12
security create-keychain -p mysecretpassword build.keychain
security default-keychain -s build.keychain
security unlock-keychain -p mysecretpassword build.keychain
security import /tmp/certificate.p12 -P "" -k build.keychain -T /usr/bin/codesign
security find-identity -v
node "$ROOT/run/ci-build-app.js"
