---
title: Caption protocol and archival format
description: MUXL text tracks, compact transcript records, and live caption delivery.
---

Captions have two complementary representations: canonical WebVTT text tracks
in signed MUXL media, and compact AT Protocol transcript records for discovery,
imports, and timed text access. See the
[streamer guide](/docs/guides/start-streaming/captions/) and
[node operator guide](/docs/guides/installing/captions/) for configuration.

## Canonical MUXL text tracks

Canonical captions are **WebVTT text tracks inside the streamer-signed MUXL
segments**, not a separate Go-only archive. The origin attaches text before
signing. All nodes, including the origin, derive public canonical cues from the
text tracks of validated segments. HLS, websocket, and WebRTC outputs consume
those extracted cues rather than a parallel pre-signing canonical feed.

The MUXL text-track metadata convention is:

- **`TextTrack.Language`**: the BCP 47 tag, or `und` for an unknown language.
- **`TextTrack.Label`**: the source, `auto`, `ingest`, or `human`. Readers treat
  an empty or unrecognized label as `ingest`; it is not a free-form display name.

Tracks are declared when their first cues arrive and retain stable IDs and
immutable configuration during the ingest session. A track replaced by another
source continues with empty text segments rather than changing its metadata.
Cue pieces crossing GoP boundaries retain their identity and are clipped to the
corresponding GoP ranges.

MUXL cue times are absolute **media-timeline milliseconds**, not Unix time.
For live delivery the conversion is:

```text
cue wall-clock = signed segment startTime
               + (cue media time - reference AV decode time)
```

Never substitute a relay's receive time. Stored videos retain text-track
metadata and byte ranges; finalized VODs publish `place.stream.media.track`
records for text tracks alongside their audio/video tracks. VOD lookup reads
MUXL text cues on the video's AV timeline and merges them with indexed
transcript records. A matching transcript copy of the streamer's mastered text
is deduplicated; imports and differently authored tracks remain available.

The convention and time conversion are implemented in
[`pkg/captions/muxl.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/muxl.go);
origin attachment is in
[`pkg/media/captions_master.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_master.go).

## Compact transcript records

[`place.stream.caption.transcript`](/docs/lex-reference/caption/place-stream-caption-transcript/)
is a chunked record format adapted from Ionosphere's
`tv.ionosphere.transcript` encoding:

- **`startMs`** locates the first word relative to the subject's media start.
- **`text`** contains whitespace-separated words.
- **`timings`** is flat: each positive integer is the duration in milliseconds
  of the next word; a negative integer is a silence gap before the next word.
  Without a gap, a word begins where the previous one ended.

For example, `startMs: 1000`, `text: "Hello world"`, and
`timings: [300, -200, 400]` place “Hello” at 1000–1300 ms and “world” at
1500–1900 ms.

For a livestream subject, all chunks repeat **`mediaStart`**, the wall-clock
start of the session's first segment. `startMs` is relative to that instant.
For a video subject, `mediaStart` is absent and offsets are relative to video
start. A strong reference in `subject` identifies the livestream or video;
`track` can identify the canonical media text track being mirrored. Use the
generated reference for the remaining fields and record limits rather than
assuming a transcript fits in one record.

Canonical records are published by the origin in the **streamer's repository**
through their stored OAuth session. Imported VOD captions also live there.
Node-generated sidecars are published in the **generating node's server
repository**, with node attribution preserved through relays. Republishing a
received sidecar as the receiving node's transcript would lose that attribution.

The encoding is implemented in
[`pkg/captions/transcript/transcript.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/transcript/transcript.go),
and repository publication in
[`pkg/captions/records/publisher.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/records/publisher.go).

## Playback delivery

### HLS live and VOD

Each available track has an **`EXT-X-MEDIA:TYPE=SUBTITLES`** rendition in the
master playlist, using **`GROUP-ID="cc"`**. Every video variant references that
group with **`SUBTITLES="cc"`**. Subtitle playlists serve plain **WebVTT text
files**, aligned to video segment ranges, rather than putting MP4 text-track
bytes into a `.vtt` response. Each segment carries an **`X-TIMESTAMP-MAP`**
that aligns the local cue clock to the video's 90 kHz MPEG timestamp clock.
Cues overlapping adjacent segments may appear in both documents.

See
[`pkg/livehls/subtitles.go`](https://github.com/streamplace/streamplace/blob/main/pkg/livehls/subtitles.go)
and [`place_stream_playback_captions.go`](https://github.com/streamplace/streamplace/blob/main/pkg/spxrpc/place_stream_playback_captions.go).

### Livestream websocket

**`/api/websocket/<streamer DID>`** delivers JSON messages with
**`$type: "place.stream.caption.defs#liveCue"`**. A cue carries its track,
wall-clock `startTime`/`endTime`, text, ID, and `final` state. Clients replace
an earlier cue with the same ID rather than displaying it twice. Interim text
can be revised; canonical GoP pieces are coalesced into the same cue.

New connections receive recent final cues (roughly the last ten seconds),
not the complete transcript. Public websocket captions are gated by the
stream's published-live state; unpublished preview captions are not a public
backlog. See
[`pkg/api/websocket.go`](https://github.com/streamplace/streamplace/blob/main/pkg/api/websocket.go)
and [`pkg/captions/livecue/livecue.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/livecue/livecue.go).

### WebRTC / WHEP

WHEP sessions use a data channel labeled **`captions`**, carrying the same
`liveCue` JSON as the websocket. The viewer's SDP offer must negotiate a data
channel (`m=application`); an AV-only offer does not acquire a caption channel.
The server opens the channel and can also serve a viewer-opened channel with
that label. Public viewers receive only published-live cues; owner preview
access follows the WHEP session's viewer identity.

See
[`pkg/media/webrtc_captions.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/webrtc_captions.go).

### Node-to-node sidecars

Sidecars do not modify the streamer's signed media. On the segment-replication
websocket, `captions=1` opts into JSON text frames carrying
`place.stream.caption.sidecar#event`, alongside binary media frames. Relays
preserve the original node author and signed segment-clock timing, and honor
`allowNodeCaptions` from validated stream metadata before distribution.

## API references

Use the generated lexicon pages as the schema source of truth:

- [Caption policy](/docs/lex-reference/metadata/place-stream-metadata-captionpolicy/)
- [Track views and live/pushed cues](/docs/lex-reference/caption/place-stream-caption-defs/)
- [Transcript record](/docs/lex-reference/caption/place-stream-caption-transcript/)
- [List tracks](/docs/lex-reference/caption/place-stream-caption-listtracks/)
- [Download captions](/docs/lex-reference/caption/place-stream-caption-getcaptions/)
- [Push live captions](/docs/lex-reference/caption/place-stream-caption-pushcaptions/)
- [Import VOD captions](/docs/lex-reference/caption/place-stream-caption-importcaptions/)

`getCaptions` supports `vtt`, `srt`, and JSON cues. Video offsets count from
video start; live downloads cover the node's current window, with JSON's
`epoch` identifying its time base. `listTracks` takes a live streamer or a video
AT-URI; use the returned track IDs instead of inventing IDs from language names.
