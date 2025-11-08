#!/bin/bash

set -euo pipefail

ONE="$(realpath "$1")"
TWO="$(realpath "$2")"

cd "$(mktemp -d)"
openssl sha256 "$ONE" "$TWO"

xxd "$ONE" > "1.xxd"
xxd "$TWO" > "2.xxd"
(diff --color=always "1.xxd" "2.xxd" || true) | head -n 50

ffmpeg -y -loglevel fatal -i "$ONE" -c copy -f framemd5 "1.md5"
ffmpeg -y -loglevel fatal -i "$TWO" -c copy -f framemd5 "2.md5"
(diff --color=always "1.md5" "2.md5" || true) | head -n 20

# ffprobe -loglevel fatal -show_frames "$1" > "1.frames"
# ffprobe -loglevel fatal -show_frames "$2" > "2.frames"
# (diff --color=always "1.frames" "2.frames" || true) | head -n 20

# ffmpeg -y -loglevel fatal -i "$1" -frames:v 1 -c copy -an 1frame.h264
# ffmpeg -y -loglevel fatal -i "$2" -frames:v 1 -c copy -an 2frame.h264

# ffmpeg -y -loglevel fatal -bsf:v trace_headers -i "$1" -c copy -f null /dev/null

# ffmpeg -y -loglevel fatal -i "$1" -c copy 1tweaked.mp4
# ffmpeg -y -loglevel fatal -i "$2" -c copy 2tweaked.mp4

# ffmpeg -y -bsf:v trace_headers -i 1tweaked.mp4 -c copy -f null /dev/null 2>&1 | sed 's/.*\]//' > 1.trace_headers
# ffmpeg -y -bsf:v trace_headers -i 2tweaked.mp4 -c copy -f null /dev/null 2>&1 | sed 's/.*\]//' > 2.trace_headers

# diff --color=always "1.trace_headers" "2.trace_headers" || true

# ffmpeg -y -loglevel fatal -bsf:v  h264_redundant_pps -i "$1" -c copy 1tweaked.mp4
# ffmpeg -y -loglevel fatal -bsf:v  h264_redundant_pps -i "$2" -c copy 2tweaked.mp4

# ffmpeg -y -loglevel fatal -i 1tweaked.mp4 -c copy -f framemd5 "1tweaked.mp4.md5"
# ffmpeg -y -loglevel fatal -i 2tweaked.mp4 -c copy -f framemd5 "2tweaked.mp4.md5"
# (diff --color=always "1tweaked.mp4.md5" "2tweaked.mp4.md5" || true)

# (diff --color=always "1.trace_headers" "2.trace_headers" || true)

# ffmpeg -y -loglevel fatal -i "$1" -c:v copy -bsf:v h264_mp4toannexb -an 1.ts
# ffmpeg -y -loglevel fatal -i "$2" -c:v copy -bsf:v h264_mp4toannexb -an 2.ts
# ffmpeg -y -loglevel fatal -i 1.ts -c copy -f framemd5 "1.ts.md5"
# ffmpeg -y -loglevel fatal -i 2.ts -c copy -f framemd5 "2.ts.md5"
# (diff --color=always "1.ts.md5" "2.ts.md5" || true)