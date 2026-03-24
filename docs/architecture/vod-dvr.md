# VOD & DVR Architecture

## Overview

Streamplace is adding VOD (video on demand) and DVR (live rewind) capabilities using the MUXL archive format. This document captures the design decisions and intended architecture.

## Core Concept: The Virtual Archive

Each stream is represented as one continuously growing virtual fMP4 file in the MUXL archive format:

```
ftyp + moov (init) + [track 1 segments in order] + [track 2 segments in order] + ...
```

This is a conceptual model — the backend stores segments individually (S3 for archived data, in-memory "mempool" for recent minutes). But to any client, it looks like a single fMP4 file addressable by byte ranges.

## Content Addressing

- Archives are identified by **BDASL BLAKE-3 hashes**
- The `place.stream.video` atproto record references the archive via an **atproto blob ref** (BLAKE-3 CID, `video/mp4` MIME type)
- BLAKE-3's Merkle tree structure means content-addressed byte ranges and per-segment CIDs are semantically equivalent — a segment CID is just a named slice of the archive at a known byte range
- The atproto record is continuously updated during a live stream as the archive grows (new blob ref with new hash/size/duration)

## ATProto Record: `place.stream.video`

A lightweight pointer published to the user's repo. Contains enough metadata for any client to display the video without fetching from a Streamplace node.

```
creator:    did           (required)
createdAt:  datetime      (required)
endedAt:    datetime      (absent while live)
title:      string
catalog:    {video, audio} (codecs, resolution, channels, timescale — inline)
archive:    blob ref      (BLAKE-3 CID, video/mp4)
duration:   integer       (nanoseconds, updated as stream grows)
livestream: strongRef     (back-reference to place.stream.livestream, if applicable)
```

## XRPC Endpoints

Defined as atproto lexicons. Operate in terms of byte range requests and CID-based blob fetches.

### `place.stream.playback.getVideoPlaylist`

Returns an HLS CMAF playlist (master or media) for a video.

- **Params**: `did`, `rkey`, `track` (optional — omit for master), `start` (ms, optional), `end` (ms, optional — omit for live EVENT)
- **Output**: `application/vnd.apple.mpegurl`
- If `end` is absent or extends past the current time → `#EXT-X-PLAYLIST-TYPE:EVENT` (live DVR)
- If fully in the past → `#EXT-X-PLAYLIST-TYPE:VOD`
- Playlists can mix URI strategies per segment based on storage backend

### `place.stream.playback.getVideoBlob`

Byte range access to the virtual archive. Supports HTTP `Range` headers.

- **Params**: `did`, `rkey`
- **Output**: `video/mp4`
- The node's segment index resolves byte ranges to the right stored segment (mempool or S3)

## Playlist URI Strategies

Playlists are generated on the fly and can mix representations per segment:

```m3u8
#EXT-X-MAP:URI="https://s3.example/archive-abc.mp4",BYTERANGE="1125@0"

# Archived segments — S3 byte ranges
#EXTINF:1.001000,
#EXT-X-BYTERANGE:98114@1125
https://s3.example/archive-abc.mp4

# Recent segments — CID references to mempool
#EXTINF:1.001000,
https://node.example/xrpc/place.stream.playback.getBlob?cid=bafk...def
```

Both representations are equivalent because the BLAKE-3 hash of the segment bytes at that byte range in the archive equals the segment's standalone CID.

## Node-Side: Segment Index Cache

The segment index is a **derived cache** — rebuildable from the MUXL archive data at any time. Not canonical data. Stored in the node's index cache alongside atproto data.

```
SegmentIndex:
  Catalog:        codec/resolution/timescale per track
  InitSize:       byte length of init segment
  Tracks:         map[trackID] → ordered list of:
    DurationTicks:  segment duration in timescale ticks
    SampleCount:    number of frames/samples
    ByteLength:     segment byte size
```

From this index, the node can:

- Generate playlists for any time range (compute byte offsets from cumulative lengths)
- Resolve byte range requests to specific segments
- Compute the archive BLAKE-3 hash
- Derive bandwidth, frame rate, target duration for HLS tags

## Data Flow

```
WebRTC/WHIP ingest
  → GStreamer pipeline (h264 + opus)
  → MUXL segmenter (WASM, CBOR events)
  → For each event:
      - Update segment index cache
      - Store segment data in mempool
      - Update place.stream.video record (new hash, duration)
      - Archive older segments to S3
  → On stream end:
      - Set endedAt on place.stream.video
      - Archive remaining mempool to S3
```

## Relationship to Existing Records

- `place.stream.livestream` — announces a live stream is happening (ephemeral, real-time)
- `place.stream.segment` — individual media chunks for live relay between nodes (ephemeral, ~10 min TTL)
- `place.stream.video` — **new** — the persistent recording, continuously updated during live, becomes static VOD when stream ends. References the MUXL archive via blob ref.

A livestream can reference its video record, and vice versa. The video record outlives the livestream — segments expire, the livestream record gets `endedAt`, but the video record and its archive persist.
