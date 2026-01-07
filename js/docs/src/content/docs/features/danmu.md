---
title: Danmu Overlay
description: Add flying bullet-style chat comments to the player, or your stream
---

:::note This feature is experimental and may change in future releases. :::

[Danmu (or Danmaku)](https://en.wikipedia.org/wiki/Danmaku_subtitling) (弹幕,
"bullet curtain") is a comment style where messages fly across the video
horizontally. Originating from Niconico and Bilibili, it's a fun way to display
chat that feels more integrated with the content. Use it in your live streams to
create a more engaging viewer experience.

## What It Does

Displays chat messages as animated text that scrolls across your stream.
Messages appear in lanes and move right-to-left at configurable speeds. The
overlay is transparent so you can layer it over your video in OBS.

## Enabling Danmu in the player

In-player danmu is currently an experimental feature. To unlock it:

1. Open Settings in Streamplace
2. Enter the "About" section
3. Tap the version number 5 times
4. You'll see "You are now a developer". congrats!
5. Go back to Settings, you should now see a "Danmu" section!

From there you can:

- Toggle Danmu on/off
- Adjust opacity (0–100%)
- Change scroll speed (0.5× to 2×)
- Set number of lanes (4–20)
- Limit max simultaneous messages (5–200)

You can then enable Danmu in the player by clicking the Danmu (弹) icon in the
bottom right controls row.

## Using It in OBS

The Danmu overlay can be used as a browser source in OBS:

1. Add a Browser Source to your scene
2. Set the URL to `https://stream.place/widgets/USERNAME/danmu`
   - Replace `USERNAME` with your Bluesky handle (without the @)
3. Set width/height to match your canvas (e.g., 1920×1080)
4. Check "Shutdown source when not visible"
5. Optionally check "Refresh browser when scene becomes active"

### Customization via URL Parameters

You can customize the overlay by adding URL parameters:

- `opacity` — transparency (0–100, default 80)
- `speed` — scroll speed multiplier (default 1)
- `laneCount` — number of message lanes (default 12)
- `maxMessages` — max simultaneous messages (default 50)

Example with custom settings:

```
https://stream.place/widgets/USERNAME/danmu?opacity=90&speed=1.5&laneCount=10
```
