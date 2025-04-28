#!/bin/bash

# Generates a video with a sine wave that loops 60 times for testing the segmenter and whatnot

set -euo pipefail

DURATION="${DURATION:-60}"

ffmpeg -y \
  -f lavfi -i "aevalsrc=0.125 * sin(2 * PI * (150+(800*mod(t\,1))) * t):d=1:sample_rate=48000,aloop=loop=${DURATION}:size=48000*${DURATION}" \
  -filter_complex "
    color=c=green:s=1280x360:r=60:d=1,format=yuv420p[red1];
    color=c=blue:s=1280x360:r=60:d=1,format=yuv420p[blue];
    [red1][blue]blend=all_expr='A*(1-T)+B*T'[fadeout];
    [fadeout]loop=loop=${DURATION}:size=${DURATION}*2[colorfade];
    [0:a]asplit[audio][audio2];
    [audio2]showwaves=split_channels=1:s=1280x360:rate=25,fps=60[waveform];
    [colorfade][waveform]vstack[video];
  " \
  -map "[video]" \
  -map "[audio]" \
  -c:v libx264 \
  -preset ultrafast \
  -g 60 \
  -keyint_min 60 \
  -x264-params "keyint=60:scenecut=0:bframes=0" \
  -crf 23 \
  -c:a libopus \
  -b:a 128k \
  -t "${DURATION}" \
  output_looped.mp4

ffplay output_looped.mp4
