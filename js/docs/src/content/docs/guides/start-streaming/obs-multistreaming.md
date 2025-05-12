---
title: OBS Multistreaming
description:
  This guide walks you through configuring OBS (Open Broadcaster Software) for
  desktop streaming using Streamplace.
order: 20
---

Simultaneously streaming to Streamplace and other platforms can be easily
accomplished with plugins such as
[obs-multi-rtmp](https://github.com/sorayuki/obs-multi-rtmp). Please note the
following settings to avoid streams with heavily distorted audio:

- Protocol: WebRTC (WHIP)
- Audio Encoder: FFmpeg Opus

![alt text](obs-multistream.jpg "Title")
