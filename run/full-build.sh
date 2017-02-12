#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

cd "$ROOT"
npm install

# Linted. Cool, we're good to deploy.
gitDescribe=$(git describe --tags)
# strip the "v"
export repoVersion=${gitDescribe:1}
# ask lerna nicely to update all of our package.json files
lerna publish --skip-git --skip-npm --yes --repo-version "$repoVersion"
# and now run the linting script that updates all the Chart.yaml files
FIX_OR_ERR="FIX" lerna exec $(realpath "$ROOT/run/package-lint.sh")
# Cool. With that, we're good to build. First publish the new version of the npm packages...
lerna publish --skip-git --yes --repo-version "$repoVersion"
# Sweet, time for Docker!
lerna exec --concurrency=1 $(realpath "$ROOT/run/package-docker-build.sh")
lerna exec --concurrency=1 $(realpath "$ROOT/run/package-docker-push.sh")
