---
title: Troubleshooting
description: js
sidebar:
  order: 40
---

### Stream is not connecting, with no specific issues

The most likely culprit is that WebRTC may be blocked to our IP. These may or
may not fix the issue:

- Switch off low latency in Video Settings -> Quality
- In Chrome derivatives you may have to set `#webrtc-ip-handling-policy` to
  Default in [`chrome://flags`](chrome://flags)

### "RTPSender created with no codecs" on Firefox

In [`about:config`](about:config) you will need to set
`media.webrtc.hw.h264.enabled` to `true`
