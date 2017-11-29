#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

/usr/bin/Xvfb $DISPLAY -ac -screen 0 $XVFB_SCREENSIZE -nolisten tcp &
node_modules/.bin/electron dist/sp-compositor.js
