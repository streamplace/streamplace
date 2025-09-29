#!/bin/bash

set -euo pipefail

files="$(find "$1" -type f -name '*Z.mp4' | sort)"
dir="$(mktemp -d)"
echo '' > "concat.txt"
for file in $files; do
    echo "file '$file'" >> "concat.txt"
done
ffmpeg -f concat -safe 0 -i "concat.txt" -c:v copy -c:a aac "$2"
rm -rf "$dir"
