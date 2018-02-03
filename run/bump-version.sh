#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

newVersion="$1"
currentVersion="$(cat "$ROOT/lerna.json" | jq -r '.version')"
info "You're about to bump streamplace from version $currentVersion to $newVersion."
info "This action will create a new git tag and push it to the repo."
confirm "Sound good?"

# Use a no-op lerna/lint operation to bump all the versions, then we do the git stuff manually.
lerna bootstrap
lerna publish --skip-git --skip-npm --yes --repo-version "$newVersion" --force-publish '*'
"$ROOT/run/lint.sh"
git add .
git commit -m "v$newVersion"
git tag "v$newVersion"

echo "Cool, done. Check the commit and tag, make sure everything looks good, then:"
echo "> git push --tags origin latest"
