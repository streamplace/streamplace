#!/bin/bash

set -euo pipefail

# go install github.com/minio/mc@latest
# mc alias set streamplace-account https://storage.googleapis.com/streamplace-account GOOGACCESS_KEY_ID SECRET_ACCESS_KEY

hash=$(openssl dgst -sha256 "$1" | awk '{ print $2 }')
dest="$hash/$(basename "$1")"
mc cp "$1" "streamplace-account/streamplace-fixtures/$dest"

echo "https://storage.googleapis.com/streamplace-fixtures/$dest"
