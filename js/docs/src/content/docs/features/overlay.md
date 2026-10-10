---
title: Overlays
description: Embed Streamplace widgets in OBS as browser sources
---

Overlays are small, self-contained widgets served at `/overlay/<name>`, built
to be added to OBS (or any other streaming app) as a **browser source**. They
draw on a transparent background, so they sit over your scene.

The URL looks like this:

```
https://stream.place/overlay/<name>?user=<your handle>
```

- `<name>` picks the widget. Run an overlay with a name that doesn't exist and
  it tells you which names are available.
- `user` is the account whose stream the overlay follows: your Bluesky handle
  (without the `@`) or your DID. It's how the overlay knows which chat and
  livestream to listen to.

Adding it to OBS:

1. Add a Browser Source to your scene.
2. Set the URL to `https://stream.place/overlay/status?user=your.handle`.
3. Set width and height to match your canvas (e.g., 1920×1080).
4. Check **Shutdown source when not visible**.

## Available overlays

### `status`

A small card showing what the overlay is connected to: the node's build, the
streamer it follows, whether the upstream websocket is connected, and the
current viewer count. It's the overlay to use while you're setting a scene up,
and it's the quickest way to tell a misconfigured URL from a broken node.

## How overlays stay current

Overlays are meant to be left running for hours at a time, so they look after
themselves:

- **They reload when the node is updated.** Every overlay polls the node's
  build manifest (`/api/version`) and reloads itself when the build changes, so
  a deploy reaches your running browser source without you restarting it. The
  poll interval defaults to 30 seconds; `?versionPollMs=5000` overrides it.
- **They hold one upstream websocket.** Each overlay keeps a live connection to
  the node (`/api/websocket/<user>`) and reconnects on its own if it drops, so
  widgets follow chat, viewer counts and stream state as they happen.

## URL parameters

| Parameter       | Default | Meaning                                           |
| --------------- | ------- | ------------------------------------------------- |
| `user`          | —       | Required. Whose stream to follow (handle or DID). |
| `versionPollMs` | `30000` | How often to check for a new build, in ms.        |

Individual widgets may accept their own parameters on top of these; the
`status` overlay doesn't take any.
