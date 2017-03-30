#!/bin/bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/../.. && pwd )"
source "$ROOT/run/common.sh"

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

filesToLint="$(git diff --cached --name-only | grep "\.js$" || echo "")"
filesToLint="$filesToLint $(git diff --cached --name-only | grep "\.jsx$" || echo "")"
if [[ -n "$filesToLint" ]]; then
  status="happy"
  for file in $filesToLint; do
    if [[ -n "$(git ls-files :$file)" ]]; then
      contents=$(git show :"$file")
      if [[ -n "$contents" ]]; then
        if (echo "$contents" | eslint --stdin --stdin-filename $file); then
          :
        else
          status="sad"
        fi
      fi
    fi
  done
  if [[ "$status" == "sad" ]]; then
    exit 1
  fi
else
  echo -en ""
fi
