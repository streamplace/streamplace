
# This was the script used to create https://www.youtube.com/watch?v=dn4uzoqWlTI

normalizePTS="setpts=PTS-STARTPTS"
normalizePTS="setpts='(RTCTIME - RTCSTART) / (TB * 1000000)'"
offsetPTS="setpts='(RTCTIME - RTCSTART) / (TB * 1000000)'"

leftFilters="$drawTS,scale=320:-1,pad=iw*2:ih"
rightFilters="$drawTS,transpose=clock,scale=320:-1"

drawTS="drawtext=fontfile=/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf: text='%{pts\:hms}': fontcolor=white@0.8: x=0: y=0: box=1: boxcolor=black"
ffmpeg \
  -thread_queue_size 512 \
  -i rtmp://oregon.ingress.stream.kitchen:1934/stream/eli \
  -thread_queue_size 512 \
  -itsoffset 00:00:06.6 \
  -i rtmp://oregon.ingress.stream.kitchen:1934/stream/iphone \
  -framerate 30 \
  -filter_complex "\
    [0:v]$leftFilters[left]; \
    [1:v]$rightFilters[right]; \
    [left][right]overlay=W/2:0[output] \
  " \
  -map "[output]" -an \
  -f flv \
  rtmp://oregon.ingress.stream.kitchen:1934/stream/output
