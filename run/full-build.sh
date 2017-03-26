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

npmTag=""
# Check if we're a tagged release version
if git "$REPO_VERSION" | grep '-'; then
  npmTag="next"
else
  npmTag="latest"
fi

if npm whoami; then
  lerna publish --exact --skip-git --skip-npm --force-publish '*' --yes --repo-version "$REPO_VERSION" --npm-tag "$npmTag"
  # and now run the linting script that updates all the Chart.yaml files
  FIX_OR_ERR="FIX" "$ROOT/run/every-package.sh" "$ROOT/run/helm-lint.sh" --concurrency=1
  # Cool. With that, we're good to build. First publish the new version of the npm packages...
  lerna publish --exact --skip-git --force-publish '*' --yes --repo-version "$REPO_VERSION" --npm-tag "$npmTag"
else
  echo "No npm auth found, not bumping package versions 'cuz I can't publish. But I'll make sure prepublish works."
  lerna run prepublish --exact
fi

# Done prepublishing. If we're CI, let's tell the apps to build.
if [[ "${CI_TRIGGER_APP_BUILDS:-}" == "true" ]]; then
  "$ROOT/run/ci-trigger-app-builds.sh"
fi

# Sweet, time for Docker!
helm init --client-only
npm run docker-build
npm run docker-push
npm run helm-build
npm run helm-push
