#!/bin/bash

set -euo pipefail

echo "downloading $STREAMPLACE_URL_LINUX_AMD64" && cd /usr/local/bin && curl -L "$STREAMPLACE_URL_LINUX_AMD64" | tar xzv
chmod +x /usr/local/bin/streamplace

exec /usr/local/bin/streamplace
