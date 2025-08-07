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

We distribute two Linux packages for Streamplace:

- `streamplace`, which is the main Streamplace binary.
- `streamplace-default-http`, which includes some additional systemd
  configuration to make Streamplace your default HTTP server on ports 80
  and 443.

If you're looking to set up a Streamplace node on a server that isn't hosting
any other services, we'd recommend e.g.
`apt install streamplace streamplace-default-http`. If your server is hosting
other HTTP servers and you'll handle the proxying yourself, you can simply
`apt install streamplace`.

### Debian/Ubuntu

```shell
sudo mkdir -p /etc/apt/keyrings
curl https://release.stream.place/streamplace.key | sudo gpg --dearmor -o /etc/apt/keyrings/streamplace.key
echo 'deb [signed-by=/etc/apt/keyrings/streamplace.key] https://release.stream.place/debian/ all main' \
  | sudo tee /etc/apt/sources.list.d/streamplace.list
sudo apt update
sudo apt install streamplace
```

## Download a binary

Binaries for all platforms are available to download from
[our GitLab server](https://git.stream.place/streamplace/streamplace/-/releases).
