#!/bin/sh

set -eu

$ANDROID_HOME/emulator/emulator @Pixel_API_28_AOSP -verbose -no-window -no-audio -gpu swiftshader_indirect -ports 5554,5555 -skip-adb-auth -no-boot-anim -show-kernel -qemu -cpu max -machine gic-version=max &
pid=$!
booted=0
while [ "$booted" != "1" ]
do
  sleep 1
  echo "Waiting for emulator..."
  booted=`adb shell getprop dev.bootcomplete` || true
done
adb shell settings put system show_touches 1
adb shell settings put system pointer_location 1

if [ "${1:-}" = "firstboot" ]; then
  kill $pid
  wait
fi
