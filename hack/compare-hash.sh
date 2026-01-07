#!/bin/bash

set -euo pipefail

ONE="$(realpath "$1")"
TWO="$(realpath "$2")"
BASE_ONE="$(basename "$ONE")"
BASE_TWO="$(basename "$TWO")"

if [[ -d "$ONE" && -d "$TWO" ]]; then
  FILES_ONE=$(find "$ONE" -maxdepth 1 -mindepth 1 -type f -name '*.mp4' | xargs -L 1 basename | sort)
  FILES_TWO=$(find "$TWO" -maxdepth 1 -mindepth 1 -type f -name '*.mp4' | xargs -L 1 basename | sort)

  NUM_FILES_ONE=$(echo "$FILES_ONE" | wc -l)
  NUM_FILES_TWO=$(echo "$FILES_TWO" | wc -l)

  if [[ "$NUM_FILES_ONE" -ne "$NUM_FILES_TWO" ]]; then
    echo "Directory contents differ: different number of files"
    comm -3 <(echo "$FILES_ONE") <(echo "$FILES_TWO")
    exit 1
  fi

  # Compare files by their order in the sorted lists, regardless of filenames
  paste <(echo "$FILES_ONE") <(echo "$FILES_TWO") | while read -r file_one file_two; do
    # skip if either file entry is empty (may only occur if line counts mismatched, but that's handled above)
    [ -n "$file_one" ] && [ -n "$file_two" ] || continue
    echo "Comparing $file_one <=> $file_two"
    set +e
    "$0" "$ONE/$file_one" "$TWO/$file_two"
    set -e
  done
  exit 0
fi


cd "$(mktemp -d)"
HASH_ONE=$(openssl sha256 "$ONE" | awk '{print $2}')
HASH_TWO=$(openssl sha256 "$TWO" | awk '{print $2}')
if [ "$HASH_ONE" = "$HASH_TWO" ]; then
  echo "Identical: $BASE_ONE $BASE_TWO $(pwd)"
  exit 0
fi
pwd
echo "Hash for $ONE: $HASH_ONE"
echo "Hash for $TWO: $HASH_TWO"

xxd "$ONE" > "1.xxd"
xxd "$TWO" > "2.xxd"
(diff --color=always "1.xxd" "2.xxd" || true) | head -n 5

ffmpeg -y -loglevel fatal -i "$ONE" -c copy -f framemd5 "1.md5"
ffmpeg -y -loglevel fatal -i "$TWO" -c copy -f framemd5 "2.md5"
(diff --color=always "1.md5" "2.md5" || true) | head -n 5

ffprobe -loglevel fatal -show_frames "$ONE" > "1.frames"
ffprobe -loglevel fatal -show_frames "$TWO" > "2.frames"
(diff --color=always "1.frames" "2.frames" || true) | head -n 5

set +e
echo -e "\033[0m"
video_frames_one="$(cat 1.frames | grep media_type=video | wc -l | xargs)"
video_frames_two="$(cat 2.frames | grep media_type=video | wc -l | xargs)"
if [[ "$video_frames_one" -ne "$video_frames_two" ]]; then
  echo "Video frame count mismatch: $video_frames_one -> $video_frames_two"
fi
set -e

audio_frames_one="$(cat 1.frames | grep media_type=audio | wc -l | xargs)"
audio_frames_two="$(cat 2.frames | grep media_type=audio | wc -l | xargs)"
if [[ "$audio_frames_one" -ne "$audio_frames_two" ]]; then
  echo "Audio frame count mismatch: $audio_frames_one -> $audio_frames_two"
fi

# ffmpeg -y -loglevel fatal -i "$ONE" -frames:v 1 -c copy -an 1frame.h264
# ffmpeg -y -loglevel fatal -i "$TWO" -frames:v 1 -c copy -an 2frame.h264

bash -c "ffmpeg -y -bsf:v trace_headers -i \"$ONE\" -c copy -f null /dev/null 2>&1" | sed 's/\[trace_headers @ 0x[0-9a-f]*\]//' > 1.trace_headers
bash -c "ffmpeg -y -bsf:v trace_headers -i \"$TWO\" -c copy -f null /dev/null 2>&1" | sed 's/\[trace_headers @ 0x[0-9a-f]*\]//' > 2.trace_headers
(diff --color=always "1.trace_headers" "2.trace_headers" || true) | head -n 5

go install github.com/Eyevinn/mp4ff/cmd/mp4ff-info@latest || true
mp4ff-info "$ONE" > 1.mp4ff-info
mp4ff-info "$TWO" > 2.mp4ff-info
(diff --color=always "1.mp4ff-info" "2.mp4ff-info" || true) | head -n 5

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
echo "Compare hex hashes:"
echo "  meld $(realpath 1.xxd) $(realpath 2.xxd)"
echo "Compare framemd5 hashes:"
echo "  meld $(realpath 1.md5) $(realpath 2.md5)"
echo "Compare mp4ff-info data:"
echo "  meld $(realpath 1.mp4ff-info) $(realpath 2.mp4ff-info)"
exit 1
