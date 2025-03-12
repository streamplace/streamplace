#!/bin/bash

set -euo pipefail

# Get the current executing directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && cd .. && pwd )"

TMPDIR="$(mktemp -d)"
cd $TMPDIR

MALLOC_CONF=prof_leak:true,lg_prof_sample:0,prof_final:true LD_PRELOAD=/usr/lib/x86_64-linux-gnu/libjemalloc.so.2 \
  "$DIR/build-linux-amd64/streamplace" --no-firehose --wide-open &
STREAMPLACE_PID=$!

sleep 3

"$DIR/build-linux-amd64/streamplace" whip --count=3 --duration=90s \
  --file=$HOME/testvids/RocketLeague_1h55m_1sGOP_1080p60_NoBframes.mp4 || true

sleep 5
curl -X POST http://127.0.0.1:39090/gc
sleep 3

kill -SIGABRT "$STREAMPLACE_PID"

wait

outfile=$(realpath "$(ls)")
echo "processing $outfile"
jeprof --text "$DIR/build-linux-amd64/streamplace" "$outfile" | head -20
