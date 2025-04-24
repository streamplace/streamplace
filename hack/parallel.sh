#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
export LD_LIBRARY_PATH="$SCRIPT_DIR/build-darwin-arm64/lib/usr/local/lib/x86_64-linux-gnu"
export DYLD_LIBRARY_PATH="$SCRIPT_DIR/build-darwin-arm64/lib/usr/local/lib"

rm -rf media.test
STREAMPLACE_TEST_COUNT=1 go test -c -timeout 60s -run '^TestConcatBin$' stream.place/streamplace/pkg/media --count=1 -v

for i in {1..50}; do
  GST_DEBUG='*:5' STREAMPLACE_TEST_COUNT=1 TEST_TAG="test_output_$i.log" ./media.test -test.paniconexit0 -test.timeout=1m0s -test.run=^TestConcatBin$ -test.count=1 -test.v=true >"test_output_$i.log" 2>&1 &
done

# Wait for all background processes to complete
wait

for i in {1..50}; do
  if cat "test_output_$i.log" | grep "panic:" > /dev/null; then
    echo "test_output_$i.log: panic:"
  fi
done
