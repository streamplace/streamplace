#!/bin/sh
# Force Mach-O archive members and their index into Darwin's archive format.
# This is shared by Meson's C/C++ static libraries and Cargo's native builds.
exec llvm-ar --format=darwin "$@"
