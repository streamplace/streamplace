#!/bin/sh

set -e

# Create streamplace group if it doesn't exist
if ! getent group streamplace >/dev/null; then
    groupadd streamplace
fi

# Create streamplace user if it doesn't exist
if ! getent passwd streamplace >/dev/null; then
    useradd -r -g streamplace -d /var/lib/streamplace -s /sbin/nologin streamplace
fi

mkdir -p /var/lib/streamplace
mkdir -p /etc/streamplace
chown -R streamplace:streamplace /var/lib/streamplace

# Create default environment file if it doesn't exist
if [ ! -f /etc/streamplace/streamplace.env ]; then
    echo "# Configure your Streamplace instance by creating lines such as:" > /etc/streamplace/streamplace.env
    echo "# SP_PUBLIC_HOST=example.com" >> /etc/streamplace/streamplace.env
fi
