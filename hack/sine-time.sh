#!/bin/bash

# Generates a video with a sine wave that loops 60 times for testing the segmenter and whatnot

set -euo pipefail

ffmpeg -y \
  -f lavfi -i "aevalsrc=0.125 * sin(2 * PI * (250+(600*mod(t\,1))) * t):d=1:sample_rate=48000,aloop=loop=60:size=48000*60" \
  -filter_complex "
    color=c=green:s=1280x720:r=60:d=1,format=yuv420p[red1];
    color=c=blue:s=1280x720:r=60:d=1,format=yuv420p[blue];
    [red1][blue]blend=all_expr='A*(1-T)+B*T'[fadeout];
    [fadeout]loop=loop=60:size=120[video];
  " \
  -map "[video]" \
  -map "0:a" \
  -c:v libx264 \
  -preset ultrafast \
  -g 60 \
  -keyint_min 60 \
  -profile:v main \
  -crf 23 \
  -c:a libopus \
  -b:a 128k \
  -t 60 \
  output_looped.mp4

ffmpeg -y -vn -i output_looped.mp4 -lavfi showwaves=split_channels=1:s=640x480 output_looped.waveform.mp4

ffplay output_looped.waveform.mp4
