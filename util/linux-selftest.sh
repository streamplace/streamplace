#!/bin/bash

set -euo pipefail
set -x

# Electron refuses to run as root, so we sudo into a non-root user. Chromium
# also wants a sandbox: container seccomp blocks the namespace sandbox, so we
# pass --no-sandbox. Note: no `--` separator before --self-test — index.ts
# reads process.argv.slice(2), and any extra CLI flag (including `--`) would
# shift --self-test past slot 2 and parseArgs would reject it. So Chromium-
# tuning options have to come from the environment, not argv:
#   LIBGL_ALWAYS_SOFTWARE=1 + GALLIUM_DRIVER=llvmpipe force Mesa software
#   rendering, which in CI takes WebRTC playback from ~0% to ~97%.
# Pre-extracting the AppImage also avoids needing FUSE inside the container.

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
chmod -R o+rX squashfs-root

sudo -u testuser env AQD_NO_UPDATE=true HOME=/home/testuser \
  LIBGL_ALWAYS_SOFTWARE=1 GALLIUM_DRIVER=llvmpipe \
  xvfb-run -a --server-args="-screen 0 1280x1024x24" \
  ./squashfs-root/AppRun --no-sandbox --self-test
