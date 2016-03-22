#!/bin/bash

curl -Lo fonts.tar.gz 'https://github.com/adobe-fonts/source-code-pro/archive/2.010R-ro/1.030R-it.tar.gz'
tar xzf fonts.tar.gz
mv "source-code-pro-2.010R-ro-1.030R-it/TTF/SourceCodePro-Semibold.ttf" font.ttf
rm -rf "source-code-pro-2.010R-ro-1.030R-it" *.tar.gz

drawTS="drawtext=fontfile=$(pwd)/font.ttf: text='%{localtime\:%H\\\\\\:%M\\\\\\:%S %z}': fontcolor=white@1: fontsize=50: x=50: y=25: box=1: boxcolor=black"
drawName="drawtext=fontfile=$(pwd)/font.ttf: text='Frankfurt': fontcolor=white@1: fontsize=50: x=300: y=400: box=1: boxcolor=black"

ffmpeg \
  -f lavfi \
  -re \
  -i 'color=s=640x480:r=30:c=black' \
  -c:v libx264 \
  -f flv \
  -filter_complex "\
    [0:v]$drawTS[time]; \
    [time]$drawName[output] \
  "\
  -map '[output]' \
  'rtmp://oregon.ingress.stream.kitchen:1934/stream/frankfurt'
