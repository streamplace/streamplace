---
title: Getting Started with Streamplace Development
description: Learn how to set up your development environment for Streamplace.
sidebar:
  order: 10
---

So, you've decided to contribute to Streamplace development. Here's how you can
get started:

## Prerequisites

- [Node.js](https://nodejs.org/)
- [Yarn](https://yarnpkg.com/)
  - A way to install it is with `pnpm/npm install -g yarn` if corepack is not
    enabled in your node install.
- Go (version 1.24)
  - If you use `mise`, you can install latest Go 1.24 with
    `mise install go@prefix:1.24`
- Meson
- Ninja
- pkg-config
- Rust
- Working C and C++ compilers: `gcc` on Linux or `clang` (via Xcode) on macOS.

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
through to the Streamplace app dev server. Usually, you'll usually want to be
hacking on both of those things at once. If this isn't the case — like you're
making exclusively backend changes — and you want to launch the node with the
embedded frontend, you can override the pertinent command line argument:

```shell
make dev && ./build-darwin-arm64/streamplace --dev-frontend-proxy=""
```

### Streamplace App

The Streamplace app will require a local Streamplace node to make any
substantial changes, so you may want to start by following the Streamplace node
section. However, if you're only working on front-end changes and you're having
trouble building the node locally, you can
[download a production release of Streamplace](https://git.stream.place/streamplace/streamplace/-/releases)
and configure it to forward to the dev server with a command like:

````shell
curl -O https://git-cloudflare.stream.place/api/v4/projects/1/packages/generic/latest/VERSION/streamplace-VERSION-darwin-arm64.tar.gz
tar -xzvf streamplace-VERSION-darwin-arm64.tar.gz
./streamplace --dev-frontend-proxy=http://127.0.0.1:38081
```

Either way, once you have a local Streamplace node running, install the prerequisites with:

```shell
yarn install
````

#### Web

```shell
yarn run app start
```

#### iOS

```shell
yarn run app ios
```

#### Android

```shell
yarn run app android
```

You can also specify a physical device with something like
`yarn run app ios -d 'Stream’s iPhone'`. Note also that this command runs a full
native build of the iOS/Android app, which is not necessary in many cases: once
you have a copy of the `Devplace` app on your device or emulator, you can boot
the dev server back up with `yarn run app start`.

Note also that `react-native-webrtc`, our primary package for streaming in and
playing back on iOS/Android, doesn't work very well in the iOS Simulator. It may
work, it may crash. Physical devices preferred when possible!

### Streamplace Desktop

1. `yarn install`
2. `yarn run desktop start`

By default Streamplace Desktop will assume there's a local node to connect to,
running with something like `make dev && ./build-linux-amd64/streamplace` above.

### Streamplace Docs

You're looking at them. Boot up the dev server with:

```
yarn run docs start
```

And you can then access them at
[http://127.0.0.1:38082/docs](http://127.0.0.1:38082/docs).
