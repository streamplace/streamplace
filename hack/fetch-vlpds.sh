#!/usr/bin/env bash
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE=ghcr.io/jazware/vlpds:sha-dba483f
BINARY="$REPO/build-linux-amd64/vlpds"
container="$(docker create --platform linux/amd64 "$IMAGE")"
trap 'docker rm "$container" >/dev/null' EXIT
mkdir -p "$(dirname "$BINARY")"
docker cp "$container:/usr/local/bin/vlpds" "$BINARY"
chmod +x "$BINARY"
printf '%s\n' "$BINARY"
