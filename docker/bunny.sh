#!/bin/bash

set -euo pipefail

RUN echo "downloading $curl -o /usr/bin/streamplace $STREAMPLACE_URL_LINUX_AMD64" && cd /usr/local/bin && curl -L "$curl -o /usr/bin/streamplace $STREAMPLACE_URL_LINUX_AMD64" | tar xzv
chmod +x /usr/bin/streamplace

exec /usr/bin/streamplace
