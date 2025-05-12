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

### 1. Initial OBS Configuration

1. Launch OBS Studio
2. Navigate to Settings > Stream

### 2. Stream.place Authentication

1. Open your web browser
2. [Visit Streamplace](https://stream.place) and log in to your account
3. Navigate to the Live Dashboard
4. Click "Stream from OBS"
5. Under Bearer Token, click "Generate Stream Key"
   - The stream key will automatically copy to your clipboard

### 3. OBS Stream Settings

1. Return to OBS Settings > Stream
2. Configure the following:
   - Service: `WHIP`
   - Server: `https://stream.place`
   - Bearer Token: _Paste your copied stream key_

### 4. Output Configuration

1. Go to OBS Settings > Output
2. Configure these settings:
   - Output Mode: Select "Advanced" from dropdown
   - Navigate to Streaming Tab

#### Streaming Settings

- Audio Encoder: _(Select appropriate encoder, e.g. AAC)_
- Video Encoder: _(Select appropriate encoder, e.g. libx264/nvenc_h264)_

#### Suggested Encoder Settings

- Rate Control: `CBR`
- Keyframe Interval: `1s`
- x264 Options: `bframes=0`

## Multi-Streaming Support

OBS supports multi-streaming through two available OBS plugins:

1. **OBS Resources - Multiple RTMP Outputs**

   - [GitHub Releases - obs-multi-rtmp](https://github.com/sorayuki/obs-multi-rtmp/releases)

2. [**Aitum Multistream Plugin**](https://aitum.tv/products/multi)

## Best Practices

- Test your stream settings before going live
- Monitor your stream health during broadcasts
- Ensure stable internet connection
- Keep your OBS software updated

## Additional Resources

- [OBS Official Documentation](https://obsproject.com/docs/)
- [Stream.place Documentation](https://stream.place/docs/)
