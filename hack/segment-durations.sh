#!/bin/bash

# Splits a video stream apart into video and audio files and checks the discrepancy between them

set -euo pipefail

dir="$(mktemp -d)"
ffmpeg -i "$1" -vn -c:a copy "$dir/audio.mkv" 2>/dev/null >/dev/null
ffmpeg -i "$1" -an -c:v copy "$dir/video.mkv" 2>/dev/null >/dev/null
videoDuration=$(gst-discoverer-1.0 "$dir/video.mkv" | grep "Duration" | sed 's/  Duration: 0:00:0//')
audioDuration=$(gst-discoverer-1.0 "$dir/audio.mkv" | grep "Duration" | sed 's/  Duration: 0:00:0//')

echo "Video duration: $videoDuration"
echo "Audio duration: $audioDuration"
echo "Difference (negative means audio is longer): $(node -p "$videoDuration - $audioDuration")"
rm -rf "$dir"
