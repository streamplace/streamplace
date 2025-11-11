#!/bin/bash

set -euo pipefail

ONE="$(realpath "$1")"
TWO="$(realpath "$2")"
BASE_ONE="$(basename "$ONE")"
BASE_TWO="$(basename "$TWO")"

if [[ -d "$ONE" && -d "$TWO" ]]; then
  FILES_ONE=$(find "$ONE" -maxdepth 1 -mindepth 1 -type f | xargs basename | sort)
  FILES_TWO=$(find "$TWO" -maxdepth 1 -mindepth 1 -type f | xargs basename | sort)

  NUM_FILES_ONE=$(echo "$FILES_ONE" | wc -l)
  NUM_FILES_TWO=$(echo "$FILES_TWO" | wc -l)

  if [[ "$NUM_FILES_ONE" -ne "$NUM_FILES_TWO" ]]; then
    echo "Directory contents differ: different number of files"
    comm -3 <(echo "$FILES_ONE") <(echo "$FILES_TWO")
    exit 1
  fi

  if ! diff <(echo "$FILES_ONE") <(echo "$FILES_TWO") >/dev/null; then
    echo "Directory contents differ (filenames not matching):"
    comm -3 <(echo "$FILES_ONE") <(echo "$FILES_TWO")
    exit 1
  fi

  # Iterate by filename
  while read -r f; do
    [ -n "$f" ] || continue
    "$0" "$ONE/$f" "$TWO/$f"
  done <<< "$FILES_ONE"
  # after all sub-comparisons
  exit 0
fi


cd "$(mktemp -d)"
HASH_ONE=$(openssl sha256 "$ONE" | awk '{print $2}')
HASH_TWO=$(openssl sha256 "$TWO" | awk '{print $2}')
if [ "$HASH_ONE" = "$HASH_TWO" ]; then
  echo "Identical: $BASE_ONE $BASE_TWO"
  exit 0
fi
pwd
echo "Hash for $ONE: $HASH_ONE"
echo "Hash for $TWO: $HASH_TWO"

xxd "$ONE" > "1.xxd"
xxd "$TWO" > "2.xxd"
(diff --color=always "1.xxd" "2.xxd" || true) | head -n 20

ffmpeg -y -loglevel fatal -i "$ONE" -c copy -f framemd5 "1.md5"
ffmpeg -y -loglevel fatal -i "$TWO" -c copy -f framemd5 "2.md5"
(diff --color=always "1.md5" "2.md5" || true) | head -n 20

ffprobe -loglevel fatal -show_frames "$ONE" > "1.frames"
ffprobe -loglevel fatal -show_frames "$TWO" > "2.frames"
(diff --color=always "1.frames" "2.frames" || true) | head -n 20

# ffmpeg -y -loglevel fatal -i "$ONE" -frames:v 1 -c copy -an 1frame.h264
# ffmpeg -y -loglevel fatal -i "$TWO" -frames:v 1 -c copy -an 2frame.h264

bash -c "ffmpeg -y -bsf:v trace_headers -i \"$ONE\" -c copy -f null /dev/null 2>&1" | sed 's/\[trace_headers @ 0x[0-9a-f]*\]//' > 1.trace_headers
bash -c "ffmpeg -y -bsf:v trace_headers -i \"$TWO\" -c copy -f null /dev/null 2>&1" | sed 's/\[trace_headers @ 0x[0-9a-f]*\]//' > 2.trace_headers
(diff --color=always "1.trace_headers" "2.trace_headers" || true) | head -n 5

# ffmpeg -y -loglevel fatal -i "$ONE" -c copy 1tweaked.mp4
# ffmpeg -y -loglevel fatal -i "$TWO" -c copy 2tweaked.mp4

# ffmpeg -y -bsf:v trace_headers -i 1tweaked.mp4 -c copy -f null /dev/null 2>&1 | sed 's/.*\]//' > 1.trace_headers
# ffmpeg -y -bsf:v trace_headers -i 2tweaked.mp4 -c copy -f null /dev/null 2>&1 | sed 's/.*\]//' > 2.trace_headers

# diff --color=always "1.trace_headers" "2.trace_headers" || true

# ffmpeg -y -loglevel fatal -bsf:v  h264_redundant_pps -i "$ONE" -c copy 1tweaked.mp4
# ffmpeg -y -loglevel fatal -bsf:v  h264_redundant_pps -i "$TWO" -c copy 2tweaked.mp4

# ffmpeg -y -loglevel fatal -i 1tweaked.mp4 -c copy -f framemd5 "1tweaked.mp4.md5"
# ffmpeg -y -loglevel fatal -i 2tweaked.mp4 -c copy -f framemd5 "2tweaked.mp4.md5"
# (diff --color=always "1tweaked.mp4.md5" "2tweaked.mp4.md5" || true)

# (diff --color=always "1.trace_headers" "2.trace_headers" || true)

# ffmpeg -y -loglevel fatal -i "$ONE" -c:v copy -bsf:v h264_mp4toannexb -an 1.ts
# ffmpeg -y -loglevel fatal -i "$TWO" -c:v copy -bsf:v h264_mp4toannexb -an 2.ts
# ffmpeg -y -loglevel fatal -i 1.ts -c copy -f framemd5 "1.ts.md5"
# ffmpeg -y -loglevel fatal -i 2.ts -c copy -f framemd5 "2.ts.md5"
# (diff --color=always "1.ts.md5" "2.ts.md5" || true)

echo -e "\033[0m"
echo "Useful commands:"
echo "Compare ffprobe -show_frames data:"
echo "  meld $(realpath 1.frames) $(realpath 2.frames)"
echo "Compare frame headers:"
echo "  meld $(realpath 1.trace_headers) $(realpath 2.trace_headers)"
exit 1
