#!/bin/bash
filesToLint="$(git diff --cached --name-only | grep "\.js$")"
if [[ -n "$filesToLint" ]]; then
  echo "$filesToLint" | xargs -L 1 eslint
else
  echo -en ""
fi
