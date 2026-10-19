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

# Set this variable to your did:plc or did:web to have admin access to the node
SP_ADMIN_DIDS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost

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

Set `SP_ATPROTO_DID=did:plc:...` (or `--atproto-did`) to override the DID returned
by `/.well-known/atproto-did` on all node hosts, including the broadcaster host
in a multi-node cluster. Without an override, discovery returns the host-specific
`did:web` identity. This does not change the node's repository DIDs or DID documents.

### Reusing a former PDS hostname

A Streamplace node does not host the accounts from a PDS that previously ran at
the same hostname. If their DID documents still point there, repository reads
return HTTP 508 (Loop Detected) rather than repeatedly proxying back to the node.
Restore the PDS or update those accounts' PDS endpoints to make their records
available again.

Repository reads forwarded by Streamplace carry `X-Streamplace-Repo-Proxy`.
Reverse proxies must preserve this header: a node can serve its own repository
to a forwarded request, but will not forward that request again.

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
    ghcr.io/streamplace/streamplace
```

## Download a binary

Binaries for all platforms are available to download from
[our GitLab server](https://git.stream.place/streamplace/streamplace/-/releases).

## Local node with an upstream station

```shell
streamplace --http-addr=127.0.0.1:38080 \
  --upstream-station=did:web:stream.place
```

`SP_UPSTREAM_STATION` is the environment equivalent. Without it, normal server
indexing and relay selection remain unchanged. With it, the node discovers the
station's HTTPS OAuth origin and MoQ firehose from its DID document. There is no
fallback to `bsky.network` or the local PDS firehose.

The local node indexes only users with a successfully completed, non-revoked
local OAuth grant. Pending logins, unrelated followed users and repos left over
from a previous standalone run do not permit PDS scraping. Per-DID subscriptions
start at login and end when the last eligible grant ends. Reconnects reconcile
the same scoped users with their actual PDS; historical records remain on disk
after logout.

Local HTTP OAuth requires a loopback listener. The local node delegates to the
station using its own DPoP key; the station independently authenticates to the
PDS using its public HTTPS callback. No public callback or development-only
OAuth flag is needed on the local node.

### Preparing an upstream station

Run an `atmoq serve` bridge against the station's upstream atproto relay, with
a valid TLS certificate and reachable QUIC UDP port. Advertise its origin:

```ini
SP_ATMOQ_RELAY=moqt://firehose.example.com:443
```

The station's DID document then includes an `#atmoq` service of type
`AtprotoMoQRelay`, alongside the existing `#streamplace` HTTPS origin. Clients
fail startup if either advertisement is absent or malformed. Use a DNS name
matching the certificate: released atmoq v0.1.1 requires TLS SNI for raw MoQ,
which Go does not send for numeric-IP URLs.

Deploy the station's chained-OAuth support and MoQ advertisement before
releasing Desktop clients that default to `did:web:stream.place`.
