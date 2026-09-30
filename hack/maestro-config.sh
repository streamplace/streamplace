#!/usr/bin/env bash
# Emit an ephemeral Maestro workspace config; never maintain a scenario list.
# Optional arguments select flow files (relative to the repo, or absolute).
set -euo pipefail
export LC_ALL=C
REPO="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO/.maestro"
shopt -s nullglob

logged_out=(logged-out/*.yaml)
logged_in=(logged-in/*.yaml)
if [ $# -gt 0 ]; then
  logged_out=()
  logged_in=()
  for file in "$@"; do
    file="${file#"$REPO/"}"
    file="${file#.maestro/}"
    [ -f "$file" ] || { echo "missing Maestro flow: $file" >&2; exit 1; }
    case "$file" in
      logged-out/*.yaml) logged_out+=("$file") ;;
      logged-in/*.yaml) logged_in+=("$file") ;;
      setup/server-setup.yaml) ;;
      setup/oauth-login.yaml) login=true ;;
      *) echo "select a flow in .maestro/{logged-out,logged-in,setup}: $file" >&2; exit 1 ;;
    esac
  done
fi

flows=(setup/server-setup.yaml "${logged_out[@]}")
if [ $# -eq 0 ] || [ ${#logged_in[@]} -gt 0 ] || [ "${login:-false}" = true ]; then
  flows+=(setup/oauth-login.yaml "${logged_in[@]}")
fi

declare -A names=()
for file in "${flows[@]}"; do
  name="${file##*/}"
  name="${name%.yaml}"
  [[ "$name" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] || { echo "use a lowercase kebab-case Maestro filename: $file" >&2; exit 1; }
  [ -z "${names[$name]:-}" ] || { echo "duplicate Maestro flow name: $name" >&2; exit 1; }
  names[$name]=1
done

printf 'flows:\n'
printf '  - "%s"\n' "${flows[@]}"
printf 'executionOrder:\n  continueOnFailure: false\n  flowsOrder:\n'
for file in "${flows[@]}"; do
  name="${file##*/}"
  printf '    - "%s"\n' "${name%.yaml}"
done
