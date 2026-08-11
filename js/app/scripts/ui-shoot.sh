#!/bin/bash
# Runs a maestro UI flow against the app and saves screenshots + a video run,
# plus the end-state view hierarchy as text, into a deterministic artifacts/<flow> dir.
# This is the agent's visual feedback loop: edit code, hot-reload, re-shoot,
# then read the PNGs in artifacts/<flow>.
#
# Usage: bash scripts/ui-shoot.sh <flow>   (flow name = maestro/<flow>.yaml)
# Env:   SP_METRO_PORT / SP_APP_SCHEME / SP_BUNDLE_OVERRIDE (passed to dev-ios.sh)

set -euo pipefail
cd "$(dirname "$0")/.."
APP_DIR="$(pwd)"

FLOW="${1:-home}"
FLOW_FILE="${APP_DIR}/maestro/${FLOW}.yaml"
if [ ! -f "$FLOW_FILE" ]; then
  echo "no such flow: ${FLOW_FILE} (available: $(ls maestro | sed 's/\.yaml//g' | tr '\n' ' '))" >&2
  exit 1
fi

# Locate maestro; prompt to install it if missing. Prefer PATH, fall back to the
# home-dir install (the official installer puts it in ~/.maestro/bin and adds it
# to shell rc files, so fresh shells may not have it on PATH).
if command -v maestro >/dev/null 2>&1; then
  MAESTRO="$(command -v maestro)"
elif [ -x "$HOME/.maestro/bin/maestro" ]; then
  MAESTRO="$HOME/.maestro/bin/maestro"
else
  INSTALL_CMD='curl -Ls https://get.maestro.mobile.dev | bash'
  echo "maestro isn't installed (it drives the UI)." >&2
  echo "install with: ${INSTALL_CMD}" >&2
  # Only prompt when interactive; agents/CI get the message and exit instead of hanging.
  if [ -t 0 ] && read -r -p "install it now? [y/N] " answer && [[ "$answer" =~ ^[Yy] ]]; then
    curl -Ls https://get.maestro.mobile.dev | bash
    MAESTRO="$HOME/.maestro/bin/maestro"
  else
    exit 1
  fi
fi

OUT="${APP_DIR}/artifacts/${FLOW}"
mkdir -p "$OUT"
rm -f "$OUT"/*.png "$OUT"/run.mp4 "$OUT"/hierarchy.json

# Metro must be up and the app launched; dev-ios.sh handles both checks.
bash scripts/dev-ios.sh

# Record the whole run as video, then drive the flow from inside the artifacts
# dir so maestro writes screenshots there.
xcrun simctl io booted recordVideo --codec h264 --display internal "$OUT/run.mp4" &
REC_PID=$!
trap 'kill -INT "$REC_PID" 2>/dev/null || true' EXIT
sleep 1

set +e
cd "$OUT"
"$MAESTRO" test "$FLOW_FILE"
MAESTRO_EXIT=$?
set -e

kill -INT "$REC_PID" 2>/dev/null || true
wait "$REC_PID" 2>/dev/null || true
trap - EXIT

# Text form of the end-state UI, useful without vision (accessibility tree).
"$MAESTRO" hierarchy > "$OUT/hierarchy.json" 2>&1 || true

echo ""
echo "artifacts: ${OUT}"
echo "  screenshots: $(ls "$OUT" | grep -c '\.png$' || true) png, video: run.mp4, tree: hierarchy.json"
echo "  quick look: open ${OUT}"
exit "$MAESTRO_EXIT"
