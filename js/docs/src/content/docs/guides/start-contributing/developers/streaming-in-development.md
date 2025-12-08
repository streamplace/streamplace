---
title: Streaming in Development
description: So you have a Streamplace node. How do you stream in?
sidebar:
  order: 20
---

Now that you have a Streamplace node running locally, you'll probably need to
stream into it. While you can use OBS or browser streaming like Streamplace
users will, there are also some techniques for streaming from the command line
to save yourself some time.

## `streamplace whip`

Streamplace itself has a built-in WHIP client, capable of streaming in MP4 files
with H264 video and Opus audio. To be properly processed by Streamplace they'll
also need a regular keyframe interval and no B-Frames.
[Here's a copy of Big Buck Bunny that may work well](https://storage.googleapis.com/streamplace-crap/BigBuckBunny_1sGOP_4kp60_NoBframes.mp4).
Once you have an appropriate file, you can stream in using `streamplace whip`:

```shell
./build-darwin-arm64/streamplace whip \
  --stream-key [obs-stream-key-from-dashboard] \
  --file BigBuckBunny_1sGOP_4kp60_NoBframes.mp4
```

There are also versions of ffmpeg that have a WHIP muxer; something like this
can work if you don't have a copy of Streamplace locally

```shell
docker run -it --rm \
  -v $HOME/videos:/videos \
  docker.io/ggtoms/ffmpeg-webrtc \
  /usr/local/ffmpeg-webrtc/ffmpeg -re -i \
  /videos/BigBuckBunny_1sGOP_4kp60_NoBframes.mp4 \
  -c copy -f whip \
  http://[LAN IP ADDRESS]:38080/api/ingest/webrtc/[STREAM KEY]
```

### Streaming without the relay

By default, Streamplace will be watching the public Bluesky relay at
`wss://bsky.network` — this keeps your node informed about new stream keys and
chat messages and whatnot. However, in some cases this may be undesirable — you
probably wouldn't want to stream the firehose to an airplane, for example. So,
you can disable the relay and allow ANYONE to stream in to your Streamplace node
using:

```shell
./build-darwin-arm64/streamplace --no-firehose --wide-open
```

This should not be used in production, as it disables all filters on who is
allowed to stream into the node, but it can be useful in development. If you do
this, you can also use `streamplace whip` without the `--stream-key` parameter;
it will randomly generate one. This can be useful for stress-testing the node;
the following command will simulate 10 livestreams streaming into your local
node with 20 viewers each:

```shell
./build-darwin-arm64/streamplace whip \
  --file=BigBuckBunny_1sGOP_4kp60_NoBframes.mp4 \
  --count=10 \
  --viewers=20
```
