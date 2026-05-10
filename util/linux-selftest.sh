#!/bin/bash

set -euo pipefail
set -x

# Electron refuses to run as root, but Chromium's SUID sandbox refuses to run
# unless chrome-sandbox is owned by root with mode 4755. So extract the AppImage
# up front, fix the helper's perms, then invoke AppRun as a non-root user.
# Pre-extracting also avoids needing FUSE inside the container.

WORKDIR=/tmp/linux-selftest
rm -rf "$WORKDIR"
mkdir -p "$WORKDIR"
cd "$WORKDIR"

if ! id testuser >/dev/null 2>&1; then
  useradd -m -u 1500 testuser
fi

curl -L --fail -o streamplace-desktop.AppImage "$1"
chmod +x streamplace-desktop.AppImage
./streamplace-desktop.AppImage --appimage-extract > /dev/null
chown root:root squashfs-root/usr/lib/streamplace-desktop/chrome-sandbox
chmod 4755 squashfs-root/usr/lib/streamplace-desktop/chrome-sandbox
chmod -R o+rX squashfs-root

sudo -u testuser env AQD_NO_UPDATE=true HOME=/home/testuser \
  xvfb-run -a --server-args="-screen 0 1280x1024x24" \
  ./squashfs-root/AppRun -- --self-test
