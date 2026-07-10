#!/bin/sh

# pkg-config wrapper so `go test ./pkg/...` works with no environment variables.
#
# `make dev-setup` registers this via `go env -w PKG_CONFIG=<repo>/hack/pkg-config.sh`,
# which Go persists in its own config (~/.config/go/env) — no shell env needed,
# and gopls/IDEs pick it up too.
#
# When cgo asks for a package that streamplace's meson build provides
# (streamplacedeps itself, the glib/gstreamer modules requested by
# go-glib/go-gst, or the libav* modules requested by lpms), we add the build
# directory's pkgconfig paths so the freshly-built copies are found. Everything
# else passes through untouched. An explicitly-set PKG_CONFIG_PATH always takes
# precedence over the injected paths, so other projects that point cgo at
# their own ffmpeg/glib (e.g. go-livepeer) keep working.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
x86_64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
esac
BUILDDIR="${STREAMPLACE_BUILDDIR:-$ROOT/build-$OS-$ARCH}"

inject=no
for arg in "$@"; do
  case "$arg" in
  streamplacedeps | streamplacedeps-uninstalled) inject=yes ;;
  glib-2.0 | gobject-2.0 | gio-2.0 | gio-unix-2.0 | gthread-2.0) inject=yes ;;
  gmodule-2.0 | gmodule-no-export-2.0 | gmodule-export-2.0) inject=yes ;;
  gstreamer-*) inject=yes ;;
  libavcodec | libavdevice | libavfilter | libavformat | libavutil) inject=yes ;;
  libswscale | libswresample | libpostproc) inject=yes ;;
  esac
done

if [ "$inject" = yes ]; then
  if [ ! -d "$BUILDDIR/lib/pkgconfig" ]; then
    echo "hack/pkg-config.sh: $BUILDDIR/lib/pkgconfig not found; run 'make dev-setup' first" >&2
  fi
  PKG_CONFIG_PATH="${PKG_CONFIG_PATH:+$PKG_CONFIG_PATH:}$BUILDDIR/lib/pkgconfig:$BUILDDIR/lib/gstreamer-1.0/pkgconfig:$BUILDDIR/meson-uninstalled"
  export PKG_CONFIG_PATH

  # On Linux, --libs queries additionally get an rpath to the build dir (so
  # ld can resolve transitive .so deps at link time and binaries run without
  # LD_LIBRARY_PATH; --disable-new-dtags makes the rpath cover transitive
  # loads) plus -lm (lpms uses libm symbols but ffmpeg's .pc files only list
  # it in Libs.private). Not all cgo packages pull in streamplacedeps, whose
  # .pc carries the same flags, so every injected --libs answer needs them.
  if [ "$OS" = linux ]; then
    wantlibs=no
    for arg in "$@"; do
      case "$arg" in
      --libs) wantlibs=yes ;;
      esac
    done
    if [ "$wantlibs" = yes ]; then
      out="$(pkg-config "$@")" || exit $?
      printf '%s -lm -Wl,-rpath,%s/lib -Wl,--disable-new-dtags\n' "$out" "$BUILDDIR"
      exit 0
    fi
  fi
fi

exec pkg-config "$@"
