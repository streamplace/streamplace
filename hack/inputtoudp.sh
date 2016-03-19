#!/bin/bash

while true; do
  ffmpeg -i rtmp://oregon.ingress.stream.kitchen:1934/stream/iphone -f mpegts -an -c:v copy udp://localhost:40000
  sleep 2
done
