#!/bin/bash

set -euo pipefail
set -x

# Electron refuses to run as root, so we sudo into a non-root user. Chromium
# also wants a sandbox: K8s pod seccomp blocks the namespace sandbox, so we
# pass --no-sandbox. Note: no `--` separator before --self-test — index.ts
# reads process.argv.slice(2), and a literal `--` would shift --self-test into
# parseArgs's positional bucket and crash. Without `--`, Electron eats
# --no-sandbox itself and --self-test lands in the right slot.
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
  xvfb-run -a --server-args="-screen 0 1280x1024x24" \
  ./squashfs-root/AppRun --no-sandbox --self-test
