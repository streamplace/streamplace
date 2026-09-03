---
title: Automatic HTTPS
description: Let your node obtain and renew its own TLS certificates from Let's Encrypt.
---

A node running with `SP_SECURE=true` terminates TLS itself. Instead of
supplying a certificate and key with `SP_TLS_CERT` and `SP_TLS_KEY`, you can
let the node get them from an ACME certificate authority. Let's Encrypt is the
default.

```bash
SP_SECURE=true \
SP_ACME=true \
SP_ACME_EMAIL=ops@example.com \
SP_BROADCASTER_HOST=example.com \
streamplace
```

By turning this on you agree to the CA's terms of service.

## What the node needs

The CA validates that you control each hostname by connecting to it. The node
answers either challenge itself:

- **HTTP-01** on the plain HTTP listener (port 80). The listener otherwise
  only redirects to HTTPS, but it serves `/.well-known/acme-challenge/` first.
- **TLS-ALPN-01** on the HTTPS listener (port 443).

So DNS for every managed hostname must point at the node (or at the station's
load balancer, see below), and port 80 or 443 must be reachable from the
internet. If you run on other ports, put the node behind a proxy that
terminates TLS instead and use `SP_BEHIND_HTTPS_PROXY=true`.

## Which hostnames

The node manages a certificate for `SP_BROADCASTER_HOST`, the station's
identity. When `SP_SERVER_HOST` is set to a different name (for example
`prod-node1.example.com` alongside `example.com`), it manages a second
certificate for that name too. Add more with `SP_ACME_DOMAINS`, comma
separated, for hostnames such as a dedicated RTMPS endpoint. Every TLS
listener (HTTPS, RTMPS, the RTMPS addon) serves whichever certificate
matches the name the client asked for.

## Stations with several nodes

Certificates, the ACME account key and in-flight challenge state all live in
the state database, not on disk. With a shared Postgres `SP_DB_URL`:

- Every node serves the same certificates, and a certificate any node
  obtains is available to the rest within seconds.
- Renewal is coordinated with database locks, so only one node renews a given
  certificate at a time and the others pick up the result.
- Any node can answer a challenge another node started, so it does not
  matter which node the CA's request lands on behind a load balancer.

A node with the default sqlite database keeps its certificates to itself.

## Testing against staging

Let's Encrypt rate-limits production issuance. While you get DNS and ports
right, use the staging environment, whose certificates browsers do not trust:

```bash
SP_ACME_CA=letsencrypt-staging
```

Any other ACME directory URL works there too. Certificates obtained from
staging are stored separately from production ones, so switching back does
not require cleanup.

## Troubleshooting

Certificate management runs in the background. If a certificate cannot be
obtained, the node still starts and logs the CA's error; TLS connections for
that hostname fail until it succeeds, and the node keeps retrying with
backoff. The usual causes are DNS not pointing at the node yet, port 80 and
443 both unreachable, or a hostname that is not public (IP addresses and
`localhost` are rejected at startup).
