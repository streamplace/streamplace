#!/bin/bash

set -euo pipefail

TMPDIR=$(mktemp -d)
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
$SCRIPT_DIR/../build-linux-amd64/streamplace clip --out "$TMPDIR/combined1.mp4" "$@"
sleep 2
$SCRIPT_DIR/../build-linux-amd64/streamplace clip --out "$TMPDIR/combined2.mp4" "$@"
xxd "$TMPDIR/combined1.mp4" > "$TMPDIR/combined1.mp4.xxd"
xxd "$TMPDIR/combined2.mp4" > "$TMPDIR/combined2.mp4.xxd"
diff --color=always "$TMPDIR/combined1.mp4.xxd" "$TMPDIR/combined2.mp4.xxd"
