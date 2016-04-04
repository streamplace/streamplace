#!/bin/bash

# nodemon sometimes doesn't fully clear our the ffmpeg subproceses for some reason. I could spend
# some time diagnosing why that's true, or I could just run this script on every restart. ¯\_(ツ)_/¯

killall ffmpeg
./node_modules/.bin/babel-node src/app.js
