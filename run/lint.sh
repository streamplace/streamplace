#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

node "$ROOT/run/audit-package-versions.js" "$ROOT"

cd "$ROOT"

export FIX_OR_ERR=${FIX_OR_ERR:-FIX}

newPackage="$(cat package.json)"
correctVersion="$(cat "$ROOT/lerna.json" | jq -r '.version')"
packageVersion="$(cat "$ROOT/package.json" | jq -r '.version')"
if [[ "$packageVersion" != "$correctVersion" ]]; then
  fixOrErr "top-level package.json has version $packageVersion instead of $correctVersion"
  newPackage=$(tweak "$newPackage" ".version" "$correctVersion")
  echo "$newPackage" > package.json
fi

eslint --color --ext=jsx --ext=js .
"$ROOT/run/every-package.sh" "$ROOT/run/helm-lint.sh" --concurrency=1
