#!/bin/bash

ffmpeg \
  -f lavfi -i "testsrc=size=1920x1080:rate=30" \
  -f lavfi -i "sine=frequency=1000" \
  -c:v libx264 \
  -c:a aac \
  -t 00:00:30 \
  testvid.ts
