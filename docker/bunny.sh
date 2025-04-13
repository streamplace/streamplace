#!/bin/bash

set -euo pipefail

curl -o /usr/bin/streamplace $STREAMPLACE_URL_LINUX_AMD64
chmod +x /usr/bin/streamplace

exec /usr/bin/streamplace
