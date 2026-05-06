---
title: Live Dashboard
description: Monitor and manage your stream from the live dashboard
---

The live dashboard is your control center while you're streaming. Navigate to it from the pop-up or sidebar once you've started broadcasting, or go directly to `/live`.

It shows:

- Stream Monitor: a live preview of your ingest feed with connection quality and stream title
- Information Widget: current bitrate, resolution, FPS, codec, and viewer count with a bitrate history chart
- Multistream Status: toggle individual multistream destinations on and off
- Chat Panel: live chat with message input
- Stream Settings: announce or update your livestream, manage metadata, and moderate

The layout adjusts to your screen width. On wide screens all panels are visible at once, and on narrower screens it stacks vertically with a button to open chat in a separate window.

## Widget popouts

Each dashboard widget is also available as a standalone page, useful for floating browser windows while you stream: for example, keeping stream settings in a small window on a second monitor.

All popout pages require you to be logged in. If you aren't, the login flow will run in the same window before loading the widget.

### Stream Monitor

```
https://stream.place/widgets/stream-monitor
```

Shows the live preview of your ingest feed, connection quality indicator, and stream title. You can toggle between the live feed and a thumbnail snapshot.

### Information Widget

```
https://stream.place/widgets/info
```

Shows bitrate, resolution, FPS, codec, and viewer count, with a rolling bitrate history chart.

### Multistream Status

```
https://stream.place/widgets/multistream
```

Lists all your configured multistream destinations with their current status, and lets you toggle each one on or off without going back to the main dashboard.

### Stream Settings

```
https://stream.place/widgets/livestream
```

The full stream settings panel: announce a new livestream, update the title or metadata, and end the stream. Equivalent to the Stream Settings column in the main dashboard.

### Chat

Chat has its own popout with additional options for customization. See the [Chat Popout](/features/chat-popout) page for details.
