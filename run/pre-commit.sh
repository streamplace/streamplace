#!/bin/bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

node "$ROOT/run/audit-package-versions.js" "$ROOT"

packageDirs="$(find "$ROOT/packages" -maxdepth 1 -mindepth 1 | xargs -L 1 basename)"

changedFiles=$(git diff --cached --name-only)
changedPackages=""
for package in $packageDirs; do
  if echo $changedFiles | grep "packages/$package" > /dev/null; then
    changedPackages="$changedPackages $package"
  fi
done

for package in $changedPackages; do
  (
    export FIX_OR_ERR=ERR
    cd "$ROOT/packages/$package"
    "$ROOT/run/helm-lint.sh"
  )
done
