#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
source "$ROOT/run/common.sh"

cd "$ROOT"
npm install
lerna bootstrap

# Linted. Cool, we're good to deploy.
gitDescribe=$(git describe --tags)
# strip the "v"
export repoVersion=${gitDescribe:1}
# ask lerna nicely to update all of our package.json files

npmTag=""
# Check if we're a tagged release version
if git tag | grep $gitDescribe; then
  npmTag="latest"
  # Hack around lerna bug(?) where it refuses to publish if it thinks the tag happened already
  git tag -d $gitDescribe
else
  npmTag="prerelease"
fi

lerna publish --skip-git --skip-npm --force-publish true --yes --repo-version "$repoVersion" --npm-tag $npmTag
# and now run the linting script that updates all the Chart.yaml files
FIX_OR_ERR="FIX" lerna exec $(realpath "$ROOT/run/package-lint.sh")
# Cool. With that, we're good to build. First publish the new version of the npm packages...
lerna publish --skip-git --force-publish true --yes --repo-version "$repoVersion" --npm-tag $npmTag
# Sweet, time for Docker!
lerna exec --concurrency=99 $(realpath "$ROOT/run/package-docker-build.sh")
lerna exec --concurrency=99 $(realpath "$ROOT/run/package-docker-push.sh")
"$ROOT/run/build-chart.sh"
