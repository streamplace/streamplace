---
title: place.stream.playback.getVideoPlaylist
description: Reference for the place.stream.playback.getVideoPlaylist lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an HLS CMAF playlist for a video. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getVideoBlob, addressed by content hash.

**Parameters:**

| Name    | Type      | Req'd | Description                                                                                                             | Constraints   |
| ------- | --------- | ----- | ----------------------------------------------------------------------------------------------------------------------- | ------------- |
| `did`   | `string`  | ✅    | DID of the video creator (the repo holding the place.stream.video record).                                              | Format: `did` |
| `rkey`  | `string`  | ✅    | Record key of the place.stream.video record.                                                                            |               |
| `track` | `string`  | ❌    | Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist. |               |
| `start` | `integer` | ❌    | Start time in nanoseconds from the beginning of the video. Defaults to 0.                                               |               |
| `end`   | `integer` | ❌    | End time in nanoseconds. Omit to include all remaining content.                                                         |               |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `VideoNotFound`: No place.stream.video record indexed for this DID and rkey.
- `TrackNotFound`: The requested track ID is not present in the video's blob.
- `BlobNotFound`: The video record references a blob this node hasn't indexed an origin for.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getVideoPlaylist",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an HLS CMAF playlist for a video. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getVideoBlob, addressed by content hash.",
      "parameters": {
        "type": "params",
        "required": ["did", "rkey"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "DID of the video creator (the repo holding the place.stream.video record)."
          },
          "rkey": {
            "type": "string",
            "description": "Record key of the place.stream.video record."
          },
          "track": {
            "type": "string",
            "description": "Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist."
          },
          "start": {
            "type": "integer",
            "description": "Start time in nanoseconds from the beginning of the video. Defaults to 0."
          },
          "end": {
            "type": "integer",
            "description": "End time in nanoseconds. Omit to include all remaining content."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "VideoNotFound",
          "description": "No place.stream.video record indexed for this DID and rkey."
        },
        {
          "name": "TrackNotFound",
          "description": "The requested track ID is not present in the video's blob."
        },
        {
          "name": "BlobNotFound",
          "description": "The video record references a blob this node hasn't indexed an origin for."
        }
      ]
    }
  }
}
```
