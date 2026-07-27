---
title: Quick Start
description: Get up and streaming on Streamplace quickly.
sidebar:
  order: 1
---

This guide gets you from zero to streaming. If you get stuck, check out the full [OBS setup guide](/docs/guides/start-streaming/obs).

:::tip
You will want to check out our [community guidelines](https://blog.stream.place/3mcqwibo4ks2w) first for guidance on what you can and cannot do on Streamplace.
:::

## So, what is Streamplace?

Streamplace is a video streaming service built on top of the AT Protocol (Authenticated Transfer Protocol), the same protocol Bluesky is built on.

## Step 1: Create your account

1. Go to [stream.place](https://stream.place)
2. Click "Sign in" in the top right.
3. Use your Atmosphere credentials to log in (ex. your Bluesky handle)
   - You'll need to use your actual password here - we're using OAuth so you enter your password on your PDS. We do not receive your password at all.
4. You're done! Your stream profile is live at `stream.place/your-handle`

## Step 2: Get your stream key

1. Click **Live Dashboard** (or go to [stream.place/live](https://stream.place/live))
2. Click **Stream from OBS**
3. Click **Generate Stream Key**
4. Your key is copied to clipboard automatically

Keep this key private. It's like a password, but for your stream.

## Step 3: Configure OBS

Open OBS and go to **Settings → Stream**:

- **Service**: `Custom...`
- **Server**: `rtmps://stream.place:1935/live`
- **Stream Key**: Paste what you copied in Step 2

Then go to **Settings → Output → Streaming**:

- **Video Encoder**: `libx264` (or `NVIDIA NVENC H.264` if you have an NVIDIA GPU)
- **Rate Control**: `CBR`
- **Bitrate**: `6000` Kbps (adjust down if you drop frames)
- **Keyframe Interval**: `1`
- **x264 Options**: `bframes=0`. If there's a 'bframes' option, you'll want to have that at '0' or unchecked.

:::caution
These last two options are very important! Your viewers' experience may be choppy or otherwise subpar if you don't have them correct.
:::

## Step 4: Go live

1. In OBS, click **Start Streaming**
2. Go back to the Live Dashboard at stream.place
3. Fill in your stream title and optionally pick a thumbnail8
4. If needed, turn on content warnings. ("Metadata" tab in Stream Settings)
5. Click **Announce Livestream**
6. Your stream is now live and visible to the world!

## Next steps

- **Customize your chat**: Change your name color in Settings > Account
- **Stream to other platforms too**: Set your Twitch/YouTube URLs in Settings > Multistream Targets to push your stream there automatically. See the [Multistreaming guide](/docs/features/multistreaming) for more information
- **Improve stream quality**: See the [OBS guide](/docs/guides/start-streaming/obs) for encoder settings and troubleshooting
- **Join the Discord!**: If you need any help, or just want to chat, check out our discord at https://discord.stream.place.

### Credits

A portion of this documentation was taken from [ndroo.tv](https://bsky.app/profile/ndroo.tv)'s excellent [guide on Streamplace](https://ndroo.tv/streamplace.html#2-configuring-your-account).
