#!/usr/bin/env bash
# test-env.sh — run a command with the env cgo Go tests need against this
# checkout's dev (shared) build in build-<os>-<arch>/, e.g.:
#
#   hack/test-env.sh go test -run TestStreamTranscoder -v ./pkg/media
#
# With no arguments, prints export lines for eval:
#
#   eval "$(hack/test-env.sh)"
#
# Works with a build dir produced by `make dev-setup` or hack/seed-build-dir.sh.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

BUILDOS="${BUILDOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
BUILDARCH="${BUILDARCH:-$(uname -m | tr '[:upper:]' '[:lower:]')}"
case "$BUILDARCH" in
  aarch64) BUILDARCH=arm64 ;;
  x86_64) BUILDARCH=amd64 ;;
esac
BUILDDIR="${BUILDDIR:-build-$BUILDOS-$BUILDARCH}"
B="$ROOT/$BUILDDIR"

if [ ! -f "$B/lib/pkgconfig/streamplacedeps.pc" ]; then
  echo "error: $B has no dev build; run 'make dev-setup' or 'hack/seed-build-dir.sh'." >&2
  exit 1
fi

PKG_CONFIG_PATH="$B/lib/pkgconfig:$B/lib/gstreamer-1.0/pkgconfig:$B/meson-uninstalled"
LD_LIBRARY_PATH="$B/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
CGO_LDFLAGS="${CGO_LDFLAGS:--lm}"
# A seeded build dir contains libs whose compiled-in plugin path points at the
# donor checkout; point GStreamer at this checkout's copies explicitly.
GST_PLUGIN_PATH="$B/lib/gstreamer-1.0"

if [ "$#" -eq 0 ]; then
  echo "export PKG_CONFIG_PATH=\"$PKG_CONFIG_PATH\""
  echo "export LD_LIBRARY_PATH=\"$LD_LIBRARY_PATH\""
  echo "export CGO_LDFLAGS=\"$CGO_LDFLAGS\""
  echo "export GST_PLUGIN_PATH=\"$GST_PLUGIN_PATH\""
  exit 0
fi

export PKG_CONFIG_PATH LD_LIBRARY_PATH CGO_LDFLAGS GST_PLUGIN_PATH
exec "$@"
