#!/bin/bash

set -euo pipefail
set -x

# Electron refuses to run as root, so spin up a non-root user we can sudo into.
# The AppImage uses --appimage-extract-and-run to skip the FUSE requirement.

WORKDIR=/tmp/linux-selftest
rm -rf "$WORKDIR"
mkdir -p "$WORKDIR"

if ! id testuser >/dev/null 2>&1; then
  useradd -m -u 1500 testuser
fi

curl -L --fail -o "$WORKDIR/streamplace-desktop.AppImage" "$1"
chmod +x "$WORKDIR/streamplace-desktop.AppImage"
chown -R testuser:testuser "$WORKDIR"

sudo -u testuser env AQD_NO_UPDATE=true HOME=/home/testuser \
  xvfb-run -a --server-args="-screen 0 1280x1024x24" \
  "$WORKDIR/streamplace-desktop.AppImage" --appimage-extract-and-run -- --self-test
