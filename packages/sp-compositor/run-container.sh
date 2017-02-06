#!/bin/bash

rm -f output/output.sock

#Enumerate the sound devices in /dev/snd
SNDDEVS=$(find /dev/snd -type c)

#Enumerate DRI devices in /dev/dri. See:
#https://www.stgraber.org/2014/02/09/lxc-1-0-gui-in-containers/
#This is needed for WebGL to be able to access the video.
SNDDEVS+=" $(find /dev/dri -type c)"

SNDFLAGS=$(j=""; for i in $SNDDEVS; do j+="--device=\"$i:$i\" "; done; echo $j)


# Note
# --lxc-conf='lxc.cgroup.devices.allow = c 116:* rwm' \ #/dev/snd devices
# --lxc-conf='lxc.cgroup.devices.allow = c 226:* rwm' \ #/dev/dri devices
# grants LXC access to /dev/snd and /dev/dri


docker run --rm \
  -e URL="https://streamplace.github.io/demo-video/" \
  -e SELECTOR="canvas" \
  -v $(pwd)/output:/app/output \
  -p 5900:5900 -it --name sp-compositor --privileged streamplace/sp-compositor $*
