#!/bin/bash

set -euo pipefail

echo "downloading $STREAMPLACE_URL_LINUX_AMD64" && cd /usr/local/bin && curl -L "$STREAMPLACE_URL_LINUX_AMD64" | tar xzv
chmod +x /usr/bin/streamplace

exec /usr/bin/streamplace
