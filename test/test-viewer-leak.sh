#!/bin/bash

set -euo pipefail

GOGC=10 GOMEMLIMIT=4GiB streamplace --wide-open --no-firehose &
PID=$!
sleep 3
streamplace whip --file=/home/iameli/testvids/RocketLeague_1h55m_1sGOP_1080p60_NoBframes.mp4 --count=10

kill $PID
