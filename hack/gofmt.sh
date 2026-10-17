#!/usr/bin/env bash
set -euo pipefail

# Format project-owned Go sources, including new ones not staged yet, but not
# fetched subprojects (ignored). A tracked file may be deleted but not staged
# yet while a separate checkpoint is committed.
files=()
while IFS= read -r -d '' path; do
  if [[ -f "$path" ]]; then files+=("$path"); fi
done < <(git ls-files -z --cached --others --exclude-standard -- '*.go')
if ((${#files[@]})); then
  gofmt "$@" "${files[@]}"
fi
