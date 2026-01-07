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
    cat <<EOF > /etc/streamplace/streamplace.env
# Configure your Streamplace instance by creating lines such as:
#
# SP_BROADCASTER_HOST=example.com
#
# If you have a multi-node cluster, they'll each need different public DNS names:
#
# SP_SERVER_HOST=prod-nyc0.example.com
#
# If you want your Streamplace node handle default HTTP and HTTPS traffic for the server, uncomment these:
#
# SP_HTTP_ADDR=:80
# SP_HTTPS_ADDR=:443
# SP_SECURE=true
# Useful if your TLS cert and key aren't in the default:
#
# SP_TLS_CERT=/tls/tls.crt
# SP_TLS_KEY=/tls/tls.key
EOF
fi
