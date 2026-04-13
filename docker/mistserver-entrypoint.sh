#!/bin/sh
set -e

CONFIG="/config/mistserver.json"

# Generate a random password if MIST_PASSWORD is not set
if [ -z "$MIST_PASSWORD" ]; then
  MIST_PASSWORD=$(head -c 32 /dev/urandom | md5sum | cut -d' ' -f1)
fi

# Replace the placeholder in the config with the actual password
sed -i "s/MIST_PASSWORD_PLACEHOLDER/$MIST_PASSWORD/" "$CONFIG"

exec MistController -c "$CONFIG"
