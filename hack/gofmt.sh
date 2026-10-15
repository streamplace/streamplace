#!/usr/bin/env bash
set -euo pipefail

# Format project-owned Go sources, not fetched subprojects. A tracked file may
# be deleted but not staged yet while a separate checkpoint is committed.
files=()
while IFS= read -r -d '' path; do
  if [[ -f "$path" ]]; then files+=("$path"); fi
done < <(git ls-files -z -- '*.go')
if ((${#files[@]})); then
  gofmt "$@" "${files[@]}"
fi
