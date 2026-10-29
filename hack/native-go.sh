#!/usr/bin/env bash
set -euo pipefail

# Go's cache does not track external static-library contents. Changing a cgo
# compiler flag invalidates native-dependent packages and their cached tests,
# without rebuilding unrelated pure-Go packages with a global -a. A shared
# library (development builds) is loaded at run time, so it needs nothing.
archive="$1/libstreamplace_native.a"
shift
if [[ ! -f "$archive" ]]; then
  exec "$@"
fi
if command -v sha256sum >/dev/null 2>&1; then
  digest=$(sha256sum "$archive")
else
  digest=$(shasum -a 256 "$archive")
fi
digest=${digest%% *}
export CGO_CFLAGS="${CGO_CFLAGS:-} -DSTREAMPLACE_NATIVE_SHA256=sp_$digest"
exec "$@"
