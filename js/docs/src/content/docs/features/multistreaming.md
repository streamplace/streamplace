---
title: Multistreaming
description: Forward your Streamplace stream to other providers.
---

:::note
This guide isn't about setting up Streamplace as an OBS destination. See [OBS Multistreaming to Streamplace](/docs/guides/start-streaming/obs-multistreaming/) for information on that.
:::

Multistreaming lets you forward your Streamplace stream to multiple platforms at the same time. Instead of streaming only to Streamplace, you can forward your stream to any platform that accepts RTMP input.

## Setting up multistream targets

1. Go to **Settings** > **Streaming** > **Multistream Targets**
2. Click **Create Multistream Target**
3. Enter the RTMP or RTMPS URL from your destination platform
4. Optionally give it a name to identify it later
5. Click **Create**

### Finding your multistream URL

Different platforms will provide their own RTMP URLs. Some common examples:

- **YouTube Live**: Format `rtmp://a.rtmp.youtube.com/live2/your-stream-key`
  - Find your stream key at https://studio.youtube.com/channel/UC/livestreaming (click the copy icon in the top right corner of the 'connect your encoder to go live' box)
- **Twitch**: Format `rtmp://usw20.contribute.live-video.net/app/your-stream-key`
  - You can get a valid RTMPS url at https://help.twitch.tv/s/twitch-ingest-recommendation
  - Find your stream key at https://dashboard.twitch.tv/settings/stream (your 'primary stream key')

:::note
Your stream key should automatically be hidden once you confirm. Make sure you've entered it correctly!
:::

## Managing targets during a stream

When you're live, you can see all your multistream targets on the Live Dashboard with their current status:

- **Green (Active)**: Successfully streaming to this target
- **Yellow (Pending)**: Connecting to this target
- **Red (Error)**: Connection failed; check your URL and credentials
- **Gray (Inactive)**: This target is disabled

You can toggle any target on or off with the switch next to its name. Changes take effect immediately.

## Limits

- **Maximum targets**: 100 total per account
- **Maximum active targets**: 5 simultaneous streams

### Credits

A portion of this documentation was taken from [ndroo.tv](https://bsky.app/profile/ndroo.tv)'s [guide on Streamplace](https://ndroo.tv/streamplace.html#2-configuring-your-account).
