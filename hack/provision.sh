#!/usr/bin/env bash
#
# Take this checkout from cold to a *proven* dev environment in one command:
# container, dependencies, embedded frontends, dev binaries, Playwright
# browser, and a green e2e run.
#
#   make provision
#
# Idempotent and cheap to re-run: every step reports what it found, and the
# expensive ones are skipped when they are already up to date. Re-run it after
# a branch switch; see .claude/skills/streamplace-docker/SKILL.md for stale artifacts.
#
# It is deliberately the last thing that runs before an agent starts: the e2e
# suite passing here means the toolchain works, so any later failure is about
# the change under test rather than the environment.
#
# What it cannot promise: the embedded frontend bundle is what the e2e flows
# exercise, so a later edit under js/ still needs a rebuild before the suite
# reflects it.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
NAME="$(basename "$REPO")"

if [[ "$NAME" != streamplace* ]]; then
  echo "refusing: $REPO is not a streamplace-* checkout" >&2
  exit 1
fi

N=0
step() {
  N=$((N + 1))
  printf '\n==> [%d] %s\n' "$N" "$*"
}

# --- where do we run? -------------------------------------------------------
# Everything below the container step runs inside it. Started from inside a
# container, there is nothing to ensure and we just work in place.
CONTAINER=""
if [[ -f /.dockerenv || -f /run/.containerenv ]]; then
  echo "already inside a container; provisioning in place"
else
  step "container (hack/container.sh ensure)"
  bash hack/container.sh ensure
  CONTAINER="$(bash hack/container.sh name)"
fi

run() {
  if [[ -z "$CONTAINER" ]]; then "$@"; else docker exec -w "$REPO" "$CONTAINER" "$@"; fi
}

# --- dependencies -----------------------------------------------------------
step "dependencies (pnpm install)"
run pnpm install

# --- frontends --------------------------------------------------------------
# `make dev` skips the JS build whenever js/app/dist/index.html exists, which
# is how a branch switch leaves you with the previous branch's bundle. Rebuild
# when a bundle is missing, or when a source it is built from is newer.
#
# Only tracked files count, and only hand-authored ones. `pnpm install` runs
# the workspace `prepare` step, which rewrites the i18n JSON under every
# `public/locales/` on every run (two of them tracked, all three generated
# from js/i18n/locales/*.ftl), plus js/brand's output. Counting those would
# report the frontends stale immediately after building them.
frontends_stale() {
  [[ -f js/app/dist/index.html && -f js/web/dist/index.html ]] || return 0
  local f
  while IFS= read -r -d '' f; do
    case "$f" in
      */public/locales/*) continue ;;
    esac
    if [[ "$f" -nt js/app/dist/index.html ]]; then
      echo "  $f is newer than js/app/dist/index.html"
      return 0
    fi
  done < <(git ls-files -z -- \
    js/app/src js/app/components js/app/store js/app/public \
    js/app/app.config.ts js/app/package.json \
    js/web js/components/src js/streamplace/src js/streamplace/package.json \
    js/i18n/locales js/i18n/i18next.config.js js/brand/generate.mjs \
    package.json pnpm-lock.yaml pnpm-workspace.yaml)
  return 1
}

step "frontends (js/app, js/web)"
if frontends_stale; then
  run make app
else
  echo "up to date; skipping (rm -rf js/app/dist js/web/dist to force)"
fi

# --- binaries ---------------------------------------------------------------
step "dev build (make dev)"
run make dev

# --- e2e --------------------------------------------------------------------
step "playwright browser"
run pnpm --filter @streamplace/e2e-web install-browser >/dev/null

step "e2e (hack/e2e-web-local.sh)"
E2E_LOG="$(mktemp)"
trap 'rm -f "$E2E_LOG"' EXIT
run hack/e2e-web-local.sh 2>&1 | tee "$E2E_LOG"
E2E_RESULT="$(grep -E '[0-9]+ passed' "$E2E_LOG" | tail -1 || true)"
if [[ -z "$E2E_RESULT" ]]; then
  echo "e2e did not report a pass — the environment is not provisioned" >&2
  exit 1
fi

# --- what you now have ------------------------------------------------------
cat <<EOF

provisioned $REPO
  container   ${CONTAINER:-this container}
  build       build-linux-amd64/libstreamplace + the dev launcher
  frontends   js/app/dist, js/web/dist (embedded in the binary)
  e2e         $E2E_RESULT

next:
  make dev                 rebuild binaries (does not start a node)
  hack/e2e-web-local.sh    re-run the suite for later changes
EOF