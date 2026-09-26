---
title: Branding bundles
description: One brand for your node, your custom domains and your apps — as a directory, a zip, or an atproto record.
---

Everything under **Settings → Branding** can be moved between nodes as a
bundle: a zip file with `branding.yaml` at its root and one file per image.
Unzipped, a bundle is a **brand directory** — the same thing the app build
reads to make your iOS, Android and desktop icons — so one directory in git
can brand both your apps and your node.

## Export

From the app: Settings → Branding → **Download bundle** (admins only).

From a terminal, against the node's state database, with no login needed:

```bash
streamplace branding export branding.zip   # a bundle
streamplace branding export my-brand/      # the same, as a brand directory
```

## What's inside

```yaml
version: 1
branding: # only what is set on this node
  siteTitle: Sample Node
  primaryColor: "#11e8b2"
  navLinks: # JSON-valued keys are plain YAML
    - { label: Home, url: https://example.com, icon: home }
  socialLinks:
    - {
        label: Bluesky,
        url: https://bsky.app/profile/example.com,
        icon: bluesky,
      }
    - { label: Forum, url: https://example.com/forum, icon: socialIcon1 }
  mainLogo: mainLogo.svg # image keys name a file in the zip
  socialIcon1: socialIcon1.svg
  linkBanner: linkBanner.png
defaults: # informational: the app's default for each key
  primaryColor: "#6366f1"
  accentColor: "#8b5cf6"
```

The YAML is meant to be edited by hand. Colors are hex, enum keys accept
the same values as the Branding screen, and every value is validated on
import exactly as it would be there, so a typo is rejected by key name rather than silently stored.

## Import

From the app: **Import bundle…** shows what would be added, changed and
removed before anything is written; **Apply** commits it.

From a terminal:

```bash
streamplace branding import --dry-run branding.zip   # preview
streamplace branding import branding.zip             # apply
streamplace branding import my-brand/                # a brand directory works too
```

Import **replaces** the node's branding: keys the bundle does not mention
are removed, so importing a bundle makes the node look like the bundle and
nothing else. Pass `--merge` (or tick _Merge_ in the app) to keep settings
the bundle leaves out.

Bundles are capped at 8MB. Seeding a brand new node before anyone can sign
in is the terminal import's main job: run it once against the data
directory, then start the node.

## Building apps from a brand

Besides what the node serves, a brand carries what an app build needs.
These `app*` keys are stored and exported with the rest but never sent to
the running app:

| Key                                                             | What it is                                                                             |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `appName`                                                       | The app's name on the home screen, the desktop app and the docs (default `siteTitle`). |
| `appWordmark`                                                   | Text beside the mark in the app's lockup (default `appName`).                          |
| `appMonochrome`                                                 | `on` when `mainLogo` is one-color art using `currentColor`.                            |
| `appColors`                                                     | Icon, splash and tile colors (`iconBackground`, `splashBackground`, …).                |
| `appBundleId`                                                   | iOS bundle identifier and Android package, e.g. `com.example.live`.                    |
| `appHost`                                                       | The node the app talks to by default, and the domain its app links open.               |
| `appIcon`, `appIconForeground`, `appSplash`, `appWordmarkImage` | Art that overrides what the build draws from `mainLogo`.                               |

The build draws every icon from `mainLogo` (an SVG with a `viewBox`) and
uses `linkBanner` as the social card. Point it at a brand directory:

```bash
SP_BRAND_DIR=path/to/my-brand pnpm run brand   # regenerate icons and brand.ts
```

The repo's own `brand/` directory is the open-source Streamplace brand in
this format; see `brand/README.md`.

## Custom domains

A node can serve other hostnames under their own brand: point a domain's
DNS at the node, and an admin grants it to an account under **Settings →
Branding → Custom domains** (or `place.stream.branding.putDomain`). If the
node terminates TLS itself, it obtains a certificate for the domain on the
first request.

The domain's brand is an atproto record in the owner's repo:
`at://<owner>/place.stream.branding.brand/<hostname>`. Every branding key is
a property of it, images as blobs. The node follows the record — it pulls it
when the domain is granted, whenever the firehose shows it change, and every
15 minutes — and serves it to everyone who visits the domain. The node's own
hostname keeps the node's brand.

The owner edits the brand from the Branding screen: **Edit brand** on the
domain points the screen at it, and every change (including importing a
bundle or brand directory) is published to their repo as the record. Any
other atproto client can write the record too; the node validates it
exactly like an import.

Because the brand is a public record, the brand behind a custom domain can
be pulled into a brand directory and built into its own apps:

```bash
streamplace branding pull at://did:plc:example/place.stream.branding.brand/live.example.com my-brand/
SP_BRAND_DIR=my-brand pnpm run brand
```
