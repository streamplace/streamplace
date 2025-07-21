---
title: Self-Hosting Streamplace
description:
  Self-host a personal copy of Streamplace using Docker and systemd. For
  advanced users.
---

## Repository

You can find the files for this guide on
https://github.com/streamplace/selfhosting

## Overview

This setup creates a streaming infrastructure with three main components:

- **Caddy** (Port 80/443) → Reverse proxy that handles TLS and routes traffic
- **Streamplace** (Port 38443) → Core streaming application that manages WebRTC
  connections
- **MistServer** (Port 31935) → RTMP ingest server that forwards to Streamplace

Traffic flow: User → Caddy → Streamplace ← MistServer ← RTMP streams

## Prerequisites

- Somewhere to host the services (VPS, dedicated server, etc.)
  - Minimum 2GB RAM recommended
  - At least 1 CPU core
  - As much disk space as you can get
- Docker and Docker Compose
- systemd (Linux)
- A domain name pointed to your server

## Quick Start

1. **Clone/copy these files** to your server
2. **Edit configuration files** (see Configuration section below)
3. **Install Streamplace binary** on your host system
   ```bash
   # Download and install Streamplace (adjust version as needed)
   wget https://github.com/streamplace/streamplace/releases/download/vX.X.X/streamplace-linux-amd64
   sudo mv streamplace-linux-amd64 /usr/bin/streamplace
   sudo chmod +x /usr/bin/streamplace
   ```
4. **Start the services:**

   ```bash
   # Start Docker services
   docker-compose up -d

   # Install and start Streamplace systemd service
   sudo cp streamplace.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable streamplace
   sudo systemctl start streamplace
   ```

## Configuration

### 1. Environment Variables

Edit `docker-compose.yml` and update:

- `DOMAIN`: Your domain name
- `EMAIL`: Your email for Let's Encrypt certificates

### 2. Streamplace Service

Edit `streamplace.service` and update:

- `--public-host`: Your domain name
- `--app-bundle-id`: Your app bundle ID (optional)

### 3. Caddy Configuration

Edit `Caddyfile` and update:

- `stream.example.com`: Replace with your actual domain
- `email youremail@domain.co.uk`: Replace with your email

## TLS Certificate Management

Caddy automatically generates and manages TLS certificates. You have two options
for how Streamplace accesses them:

### Option 1: Use Caddy's internal certificates directly with Streamplace (Recommended for production)

```bash
# In your streamplace.service file, add:
--tls-cert /path/to/your/caddy_data/certificates/acme-v02.api.letsencrypt.org-directory/your-domain/your-domain.crt \
--tls-key /path/to/your/caddy_data/certificates/acme-v02.api.letsencrypt.org-directory/your-domain/your-domain.key \
```

### Option 2: Copy certificates to Streamplace directory (Recommended for simplicity)

```bash
# Create the TLS directory in Streamplace data directory
sudo mkdir -p /var/lib/streamplace/tls

# Copy certificates from Caddy (you may want to set up a script to do this automatically)
sudo cp ./caddy_data/certificates/acme-v02.api.letsencrypt.org-directory/your-domain/your-domain.crt /var/lib/streamplace/tls/tls.crt
sudo cp ./caddy_data/certificates/acme-v02.api.letsencrypt.org-directory/your-domain/your-domain.key /var/lib/streamplace/tls/tls.key
```

## Service Management

### Docker Services

```bash
# Start all Docker services
docker-compose up -d

# View logs
docker-compose logs -f caddy
docker-compose logs -f streamplace-mistserver

# Stop services
docker-compose down
```

### Streamplace Service

```bash
# Start/stop/restart
sudo systemctl start streamplace
sudo systemctl stop streamplace
sudo systemctl restart streamplace

# View status and logs
sudo systemctl status streamplace
sudo journalctl -u streamplace -f
```

## Ports

- **80/443**: HTTP/HTTPS (Caddy)
- **31935**: RTMP (MistServer)
- **38443**: Streamplace HTTPS (internal, proxied by Caddy)
- **39090**: Streamplace internal API (localhost only)

## Troubleshooting

### Common Issues

1. **Certificates not working**: Check that your domain DNS points to your
   server and Caddy has generated your certificates correctly.

2. **RTMP not working**: Ensure MistServer is running and can reach the
   Streamplace internal API at `host.docker.internal:39090`

3. **Service won't start**: Check logs with `journalctl -u streamplace -f` and
   ensure the Streamplace binary is installed at `/usr/bin/streamplace`

### Logs

- **Caddy logs**: `docker-compose logs caddy`
- **MistServer logs**: `docker-compose logs streamplace-mistserver`
- **Streamplace logs**: `sudo journalctl -u streamplace -f`

## Security Notes

- The setup uses `tls_insecure_skip_verify` in Caddy config for the internal
  connection to Streamplace. It's worth noting that doing this skips TLS
  verification for the caddy <> streamplace connection.
- Ensure your firewall only allows necessary ports (80, 443, 1935)
  - Or at least restrict access to the RTMP port (31935) and internal API port
    (39090) to known IPs
  - These known IPs (at least for the internal API port) should INCLUDE your
    Mistserver container IP
- Consider using Tailscale or similar for remote management access
- Regularly update all components (Streamplace, Caddy, MistServer)

## Advanced Configuration

### Custom Streamplace Flags

Common flags you might want to change in `streamplace.service`:

- `--rate-limit-per-second 10`: Rate limiting
- `--rate-limit-burst 20`: Burst rate limiting
- `--secure`: Enable secure mode
- `--app-bundle-id tv.your.app`: Custom app bundle ID
- the `--tls-cert` and `--tls-key` flags for custom TLS certificates, mentioned
  above

### MistServer Configuration

Edit `mistserver.json` to customize:

- RTMP port (currently 31935)
- HTTP port (currently 28080)
- Stream processing settings
- Authentication settings
