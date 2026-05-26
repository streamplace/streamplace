---
title: place.stream.playback.getLiveSegment
description: Reference for the place.stream.playback.getLiveSegment lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Fetch a single live HLS segment, or a track's init segment, from the in-memory live window. `seg` is `init` for the EXT-X-MAP init segment, otherwise the segment's media-sequence number; a cosmetic `.m4s` suffix is accepted (and ignored) so ffmpeg-based HLS players will fetch it. HTTP Range is honored. Segments are the verbatim signed canonical .m4s, so provenance travels with playback.

**Parameters:**

| Name       | Type     | Req'd | Description                                                                                                                                                          | Constraints |
| ---------- | -------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `streamer` | `string` | ✅    | The streamer: a DID or a Bluesky handle (resolved to its DID).                                                                                                       |             |
| `track`    | `string` | ✅    | Track ID (stringified u32 matching the MUXL container).                                                                                                              |             |
| `seg`      | `string` | ✅    | `init` for the track's init segment, or the segment's media-sequence number. A trailing `.m4s` is accepted and ignored.                                              |             |
| `sid`      | `string` | ❌    | Opaque playback session identifier, propagated from the media playlist that referenced this segment. Logged for view-count correlation; not used for access control. |             |

**Output:**

- **Encoding:** `video/mp4`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `StreamNotLive`: No live window for this streamer on this node.
- `SegmentNotFound`: The requested segment has slid out of the window, or never existed.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getLiveSegment",
  "defs": {
    "main": {
      "type": "query",
      "description": "Fetch a single live HLS segment, or a track's init segment, from the in-memory live window. `seg` is `init` for the EXT-X-MAP init segment, otherwise the segment's media-sequence number; a cosmetic `.m4s` suffix is accepted (and ignored) so ffmpeg-based HLS players will fetch it. HTTP Range is honored. Segments are the verbatim signed canonical .m4s, so provenance travels with playback.",
      "parameters": {
        "type": "params",
        "required": ["streamer", "track", "seg"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "The streamer: a DID or a Bluesky handle (resolved to its DID)."
          },
          "track": {
            "type": "string",
            "description": "Track ID (stringified u32 matching the MUXL container)."
          },
          "seg": {
            "type": "string",
            "description": "`init` for the track's init segment, or the segment's media-sequence number. A trailing `.m4s` is accepted and ignored."
          },
          "sid": {
            "type": "string",
            "description": "Opaque playback session identifier, propagated from the media playlist that referenced this segment. Logged for view-count correlation; not used for access control."
          }
        }
      },
      "output": {
        "encoding": "video/mp4"
      },
      "errors": [
        {
          "name": "StreamNotLive",
          "description": "No live window for this streamer on this node."
        },
        {
          "name": "SegmentNotFound",
          "description": "The requested segment has slid out of the window, or never existed."
        }
      ]
    }
  }
}
```
