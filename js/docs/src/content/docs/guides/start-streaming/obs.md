---
title: Start streaming with OBS
description:
  This guide walks you through configuring OBS (Open Broadcaster Software) for
  desktop streaming using Streamplace.
order: 10
---

### Prerequisites

- [OBS Studio](https://obsproject.com/download) installed on your computer
- An AT Protocol account (same as your Bluesky account) for logging in to
  Streamplace
- Web browser

## Basic Setup Instructions

### 1. Get your Stream Key from Streamplace

1. Open your web browser
2. [Visit Streamplace](https://stream.place) and log in to your account
3. Navigate to the Live Dashboard
4. Click "Stream from OBS"
5. Select either `WHIP` (preferred) or `RTMP`.
6. Click "Generate Stream Key"
   - The stream key will automatically be copied to your clipboard

### 2. Configure OBS Studio

#### 2a. Initial OBS Configuration

1. Launch OBS Studio
2. Navigate to Settings > Stream

#### 2b. Stream Settings

1. Return to OBS Settings > Stream
2. Configure the following:
   - Service:
     - If using `WHIP`, select `WHIP`.
     - If using `RTMP`, select `Custom...`.
   - Server:
     - If using `WHIP`: `https://stream.place`
     - If using `RTMP`: `rtmps://stream.place:1935/live`
   - Bearer Token (for WHIP) or Stream Key (for RTMP): _Paste your copied stream
     key_

#### 2c. Output Configuration

1. Go to OBS Settings > Output
2. Configure these settings:
   - Output Mode: Select "Advanced" from dropdown
   - Navigate to Streaming Tab

#### 2d. Streaming Settings

- Audio Encoder:
  - For `WHIP`, use `ffmpeg_opus`.
  - For `RTMP`, choose an appropriate AAC encoder.
- Video Encoder: _(Select appropriate encoder, e.g. libx264/nvenc_h264)_

#### 2e. Suggested Video Encoder Settings

- Rate Control: `CBR`
- Keyframe Interval: `1s`
- x264 Options: `bframes=0`
  - If available, there also may be a 'no bframes' checkbox which should be
    checked

### 3. Announce your stream

1. Once you're live, go back to the live dashboard.
2. There, you can fill out your stream title and choose an optional thumbnail.
3. Click 'Announce Livestream' to announce your livestream to the world!

## Multi-Streaming Support

OBS supports multi-streaming through two available OBS plugins:

1. **OBS Resources - Multiple RTMP Outputs**

   - [GitHub Releases - obs-multi-rtmp](https://github.com/sorayuki/obs-multi-rtmp/releases)
   - [OBS Multistreaming Guide](guides/obs-multistreaming)

2. [**Aitum Multistream Plugin**](https://aitum.tv/products/multi)

## Best Practices

- Test your stream settings before going live
- Monitor your stream health during broadcasts
  - If you see lots of dropped frames, lower your bitrate.
- Ensure stable internet connection
- Keep your OBS software updated

## Additional Resources

- [OBS Official Documentation](https://obsproject.com/docs/)
