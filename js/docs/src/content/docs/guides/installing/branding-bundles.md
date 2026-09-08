---
title: Branding bundles
description: Export a node's branding as a zip, edit it by hand, and import it on another node.
---

Everything under **Settings → Branding** can be moved between nodes as a
bundle: a zip file with `branding.yaml` at its root and one file per image.

## Export

From the app: Settings → Branding → **Download bundle** (admins only).

From a terminal, against the node's state database, with no login needed:

```bash
streamplace branding export branding.zip
```

## What's inside

```yaml
version: 1
branding: # only what is set on this node
  siteTitle: Sample Node
  primaryColor: "#11e8b2"
  streamLayout: card
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

The YAML is meant to be edited by hand. Colors are hex, enum keys such as
`streamLayout` and `typeface` accept the same values as the Branding
screen, and every value is validated on import exactly as it would be
there, so a typo is rejected by key name rather than silently stored.

## Import

From the app: **Import bundle…** shows what would be added, changed and
removed before anything is written; **Apply** commits it.

From a terminal:

```bash
streamplace branding import --dry-run branding.zip   # preview
streamplace branding import branding.zip             # apply
```

Import **replaces** the node's branding: keys the bundle does not mention
are removed, so importing a bundle makes the node look like the bundle and
nothing else. Pass `--merge` (or tick _Merge_ in the app) to keep settings
the bundle leaves out.

Bundles are capped at 8MB. Seeding a brand new node before anyone can sign
in is the terminal import's main job: run it once against the data
directory, then start the node.
