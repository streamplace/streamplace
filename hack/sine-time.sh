#!/bin/bash

# Generates a video with a sine wave that loops 60 times for testing the segmenter and whatnot

set -euo pipefail

DURATION="${DURATION:-60}"
WIDTH="${WIDTH:-1280}"
HEIGHT="${HEIGHT:-720}"
HALF_HEIGHT=$((HEIGHT / 2))

ffmpeg -y \
  -f lavfi -i "aevalsrc=0.125 * sin(2 * PI * (150+(800*mod(t\,1))) * mod(t\,1)):sample_rate=48000" \
  -filter_complex "
    color=c=green:s=${WIDTH}x${HALF_HEIGHT}:r=60:d=${DURATION},format=yuv420p[green];
    color=c=blue:s=${WIDTH}x${HALF_HEIGHT}:r=60:d=${DURATION},format=yuv420p[blue];
    [green][blue]overlay=x='mod(((n-1)/60),1)*(overlay_w+(overlay_w/60)+1)'[colorfade];
    [0:a]asplit[audio][audio2];
    [audio2]showwaves=split_channels=1:s=${WIDTH}x${HALF_HEIGHT}:rate=25,fps=60[waveform];
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
