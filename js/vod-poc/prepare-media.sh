#!/usr/bin/env bash
# Generate the proof of concept's renditions into DIR: a 240p starter small
# enough to inline its first keyframe, and a 1080p rendition to switch up to.
# Both are H.264 without B-frames, with an access unit delimiter per frame and
# a keyframe every second, so groups line up across renditions.
#
#   js/vod-poc/prepare-media.sh DIR [SECONDS]
set -euo pipefail

out="$1"
seconds="${2:-30}"
mkdir -p "$out"

# A synthetic 1080p source, with a little noise so bitrates are honest.
src="testsrc2=size=1920x1080:rate=30:duration=$seconds,noise=alls=8:allf=t"
x264=(-c:v libx264 -preset medium -bf 0 -g 30 -keyint_min 30 -sc_threshold 0
  -x264-params aud=1 -an -f h264)

ffmpeg -hide_banner -loglevel error -y -f lavfi -i "$src" \
  -vf scale=426:240 "${x264[@]}" -profile:v main \
  -b:v 150k -maxrate 150k -bufsize 60k "$out/starter.h264"
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "$src" \
  "${x264[@]}" -profile:v high \
  -b:v 4M -maxrate 5M -bufsize 4M "$out/hd.h264"

cat > "$out/renditions.json" <<EOF
[
  {"name": "starter", "file": "starter.h264", "width": 426, "height": 240, "fps": 30},
  {"name": "hd", "file": "hd.h264", "width": 1920, "height": 1080, "fps": 30}
]
EOF
