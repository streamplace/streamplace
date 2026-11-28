#!/bin/sh
# streamplacedeps is a link-only cgo package: its C translation unit includes
# no headers. Returning every transitive GStreamer include directory for this
# one query can exceed Linux's per-string limit in GCC's COLLECT_GCC_OPTIONS.
# Other cgo packages still need their own normal compile flags.
if [ "$#" = 3 ] && [ "$1" = --cflags ] && [ "$2" = -- ] && [ "$3" = streamplacedeps-uninstalled ]; then
  exit 0
fi
exec pkg-config "$@"
