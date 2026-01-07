---
title: Getting Started with Streamplace Development
description: Learn how to set up your development environment for Streamplace.
sidebar:
  order: 10
---

So, you've decided to contribute to Streamplace development. Here's how you can
get started:

## Prerequisites

Except for the C/C++ compilers, we'd highly recommend using
[mise](https://mise.jdx.dev/) to get your workspace set up for development.

- [Node.js](https://nodejs.org/) (version 22 [important!])
- [pnpm](https://pnpm.io/)
- Go (version 1.24)
- Rust
- Meson
- Ninja
- pkg-config
- Working C and C++ compilers: `gcc` on Linux or `clang` (via Xcode) on macOS.
  - On most unix-like systems, a c/++ compiler is included with the distro's
    version of `build-essential`/`base-devel` (`xcode-select –-install` on
    macOS)

## Get Started

### Streamplace Node

1. Install prereqs
2. `make dev-setup`

Now you're ready to start developing! The app can be rebuilt with `make dev`, so
as you make changes to the node, you can re-run your app something like this:

macOS:

```shell
make dev && ./build-darwin-arm64/streamplace
```

Linux:

```shell
make dev && ./build-linux-amd64/streamplace
```

The node will be accessible at [http://127.0.0.1:38080](http://127.0.0.1:38080).

By default, the `make dev` Streamplace node will proxy incoming requests
front-end requests — basically every endpoint that's not at `/api` or `/xrpc` —
through to the Streamplace app dev server. Usually, you'll want to be hacking on
both of those things at once. If this isn't the case — like you're making
exclusively backend changes — and you want to launch the node with the embedded
frontend, you can override the pertinent command line argument:

```shell
make dev && ./build-darwin-arm64/streamplace --dev-frontend-proxy=""
```

If you're using a proxy server, you may want to set your tunnel URL as the
public host URL so you can get authentication working. You may do that via the
`--broadcaster-host` argument.

Similarly, if you're working on mobile and need authentication, use the
`--app-bundle-id` argument with your bundle NSID in `app.json` (for Devplace,
the id is `tv.aquareum.dev`).

Here's an example with both:

```shell
make dev && ./build-darwin-arm64/streamplace \
  --broadcaster-host your.proxy.example.com \
  --app-bundle-id tv.aquareum.dev
```

### Streamplace App

The Streamplace app will require a local Streamplace node to make any
substantial changes, so you may want to start by following the Streamplace node
section. However, if you're only working on front-end changes and you're having
trouble building the node locally, you can
[download a production release of Streamplace](https://git.stream.place/streamplace/streamplace/-/releases)
and configure it to forward to the dev server with a command like:

```shell
curl -O https://git-cloudflare.stream.place/api/v4/projects/1/packages/generic/latest/VERSION/streamplace-VERSION-darwin-arm64.tar.gz
tar -xzvf streamplace-VERSION-darwin-arm64.tar.gz
./streamplace --dev-frontend-proxy=http://127.0.0.1:38081
```

Either way, once you have a local Streamplace node running, install the
prerequisites with:

```shell
pnpm install
```

Then start building all of the packages with:

```shell
pnpm run start
```

#### iOS Build

```shell
pnpm run app ios
```

#### Android Build

```shell
pnpm run app android
```

You can also specify a physical device with something like
`pnpm run app ios -d 'Stream’s iPhone'`. Note that these commands only run
native builds; you'll still need the development server booted up with
`pnpm run start`.

Note also that `react-native-webrtc`, our primary package for streaming in and
playing back on iOS/Android, doesn't work very well in the iOS Simulator. It may
work, it may crash. Physical devices preferred when possible!

### Streamplace Docs

You're looking at them. Boot up the dev server with:

```
pnpm run docs start
```

And you can then access them at
[http://127.0.0.1:38082/docs](http://127.0.0.1:38082/docs).

## Reverse Proxy

For testing certain applications of the Streamplace node, a reverse proxy may be
necessary to handle incoming HTTPS requests from the public internet. To test
these use cases, you'll need to run the streamplace node with something like:

```shell
--dev-public-oauth=false \
--broadcaster-host=yourproxy.example.com
```

This will break logging in at `http://127.0.0.1:38080` but allow you to log in
through your public HTTPS address. Popular options include:

- [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
  (recommended)
- [zrok](https://zrok.io/) (self hostable, recommended)
- [Pangolin](https://github.com/fosrl/pangolin) (self-hostable)
- [ngrok](https://ngrok.com/)

**Example usage:**

- Cloudflare Tunnel: `cloudflared tunnel --url http://127.0.0.1:38080`
- zrok: `zrok share http 127.0.0.1:38080`
- Pangolin: (if you have a site set up)
  `newt --id my-id --secret my-secret --endpoint 127.0.0.1:38080`
- ngrok: `ngrok http 38080`

> **Tip:** A static tunnel URL is preferred for consistency, especially if you
> need to share your dev environment or if you want to stay logged in between
> proxy restarts. Look at the docs for your preferred reverse proxy for more
> information.
