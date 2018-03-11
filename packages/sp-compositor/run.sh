#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

/usr/bin/Xvfb $DISPLAY -nocursor -ac -screen 0 $XVFB_SCREENSIZE -nolisten tcp &
unclutter & # hides mouse cursor
node_modules/.bin/electron --enable-logging dist/sp-compositor.js $*
exit 1
