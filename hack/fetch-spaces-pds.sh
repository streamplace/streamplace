#!/usr/bin/env bash
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
pnpm --dir "$REPO/js/dev-env/spaces-pds" install --frozen-lockfile
