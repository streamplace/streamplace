#!/bin/bash
set -euo pipefail

# Smoke test: run `streamplace vod-test` against a list of fixture VODs
# using a built static binary. Used in CI right after `make linux-amd64`
# to catch crashes (gstreamer plugin bugs, codec mishandling, etc.)
# that the Go test suite can miss when it isn't actually linking against
# the static gstreamer-full plugin set.
#
# Each fixture is "<sha256>/<filename>", matching the layout used by
# pkg/test/remote/RemoteFixture. Files are cached at
# ~/.streamplace-test-cache/<hash>/<filename>, same as the Go helper.
#
# Skips cleanly if the binary isn't present — we only run on the
# linux-amd64 build host; other targets cross-compile and can't
# execute the produced binary.

BIN="${1:-./build-linux-amd64/streamplace}"
CACHE_DIR="${HOME}/.streamplace-test-cache"
FIXTURE_URL="https://storage.googleapis.com/streamplace-fixtures"

FIXTURES=(
	"8d03302f5fcf2b5e13e2be48af5340dbd28b2d77ac1875a693c52adff45438c4/THE-FINALS-2026-05-16-7-44-18-PM.h264.tiny.10s.mp4"
	"eb34493396ec3606e3f6d5543c7f695b6ec4b8959c083ffbd04623ea16ed9598/RocketLeague_30s_1sGOP-1080p60_NoBframes.mp4"
)

if [ ! -x "$BIN" ]; then
	echo "test-vod: $BIN not found, skipping (cross-compiled target?)"
	exit 0
fi

mkdir -p "$CACHE_DIR"

for fixture in "${FIXTURES[@]}"; do
	hash="${fixture%%/*}"
	filename="${fixture#*/}"
	target_dir="$CACHE_DIR/$hash"
	target_path="$target_dir/$filename"

	if [ ! -f "$target_path" ]; then
		echo "=== test-vod: downloading $filename ==="
		mkdir -p "$target_dir"
		tmp_path="$target_path.download"
		curl -fsSL "$FIXTURE_URL/$fixture" -o "$tmp_path"
		actual_hash=$(sha256sum "$tmp_path" | awk '{print $1}')
		if [ "$actual_hash" != "$hash" ]; then
			rm -f "$tmp_path"
			echo "test-vod: hash mismatch for $filename: expected $hash got $actual_hash" >&2
			exit 1
		fi
		mv "$tmp_path" "$target_path"
	fi

	echo "=== test-vod: $filename ==="
	"$BIN" vod-test "$target_path"
done

echo "test-vod: all fixtures passed"
