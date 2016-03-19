#!/bin/bash

rtmpInput="movie='udp\://localhost\:40000':s=dv:loop=0"
scale="scale=320:240"
genPTS="setpts='(RTCTIME - RTCSTART) / (TB * 1000000)'"
transpose="transpose=clock"

# ffmpeg \
#   -f lavfi \
#   -re \
#   -i 'color=s=320x240:r=30:c=red' \
#   -thread_queue_size 512 \
#   -i 'rtmp://oregon.ingress.stream.kitchen:1934/stream/iphone' \
#   -c:v libx264 \
#   -f flv \
#   -filter_complex "\
#     [1:v]scale=320:240[stream]; \
#     [0:v][stream]overlay=0:0:eof_action=pass[output] \
#   "\
#   -map '[output]' \
#   -tune zerolatency \
#   -vsync drop \
#   'rtmp://oregon.ingress.stream.kitchen:1934/stream/output'



ffmpeg \
  -f lavfi \
  -re \
  -i 'color=s=320x240:r=30:c=black' \
  -c:v libx264 \
  -f flv \
  -filter_complex "\
    $rtmpInput,$transpose,$scale,$genPTS[stream]; \
    [0:v][stream]overlay=0:0:eof_action=pass[output] \
  "\
  -map '[output]' \
  -tune zerolatency \
  'rtmp://oregon.ingress.stream.kitchen:1934/stream/output'
