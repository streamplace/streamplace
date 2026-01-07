---
title: Downloading Streamplace
description: How to download Streamplace
sidebar:
  order: 20
---

## macOS

```shell
brew install streamplace/streamplace/streamplace
```

## Linux

### Debian/Ubuntu

```shell
sudo mkdir -p /etc/apt/keyrings
curl https://release.stream.place/streamplace.key | sudo gpg --dearmor -o /etc/apt/keyrings/streamplace.key
echo 'deb [signed-by=/etc/apt/keyrings/streamplace.key] https://release.stream.place/debian/ all main' \
  | sudo tee /etc/apt/sources.list.d/streamplace.list
sudo apt update
sudo apt install streamplace
```

This will install the `streamplace` systemd service. To configure it, you will
want to edit the environment variables at `/etc/streamplace/streamplace.env`. An
example production env file might look something like this:

```ini
# Handle default HTTP and HTTPS traffic for the server
SP_HTTP_ADDR=:80
SP_HTTPS_ADDR=:443
SP_SECURE=true

# If you're running Streamplace behind an HTTPS proxy, you'll want
# SP_SECURE=false
# SP_BEHIND_HTTPS_PROXY=true

# Necessary to advertise a public Streamplace broadcaster
SP_BROADCASTER_HOST=example.com
# If you have a multi-node cluster, they'll each need different public DNS names:
SP_SERVER_HOST=prod-nyc0.example.com

# If you don't want to syndicate everyone, add your list of allowed DIDs here:
SP_ALLOWED_STREAMS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost

# Useful if your TLS cert and key aren't in the default
SP_TLS_CERT=/tls/tls.crt
SP_TLS_KEY=/tls/tls.key
```

### Docker

Running Streamplace from a Docker image works great except for Docker
networking. Streamplace relies heavily on WebRTC for playback, which requires
large numbers of ephemeral UDP ports to work properly. So, we recommend using
host networking. So that command would look something like:

```shell
  docker run \
    --name streamplace \
    -d \
    -e SP_HTTP_ADDR=:80 \
    -e SP_HTTPS_ADDR=:443 \
    -e SP_SECURE=true \
    -e SP_BROADCASTER_HOST=example.com \
    -e SP_ALLOWED_STREAMS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost \
    -v /var/lib/streamplace:/var/lib/streamplace \
    --net=host \
    oci.stream.place/streamplace
```

## Download a binary

Binaries for all platforms are available to download from
[our GitLab server](https://git.stream.place/streamplace/streamplace/-/releases).
