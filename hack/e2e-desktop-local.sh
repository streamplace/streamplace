#!/usr/bin/env bash
# Run only Electron Desktop flows against the isolated station + dummy PDS.
# Usage: hack/e2e-desktop-local.sh [-- <extra Playwright args>]
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$PWD"
[ "${1:-}" != "--" ] || shift

# Desktop uses the upstream station for OAuth, not the full-node web topology.
export SP_E2E_UPSTREAM_STATION=true
export SP_E2E_INGEST=none
if ! command -v "${SP_E2E_ATMOQ_BIN:-atmoq}" >/dev/null 2>&1; then
  echo "atmoq missing — see js/desktop/README.md for Desktop E2E prerequisites" >&2
  exit 1
fi
# shellcheck source=hack/lib/e2e-harness.sh
. hack/lib/e2e-harness.sh
e2e_harness_start

# Keep the shell's EXIT trap: do not exec the test runner.
cd "$REPO/js/e2e-web"
pnpm exec playwright test --config playwright.desktop.config.ts "$@"
