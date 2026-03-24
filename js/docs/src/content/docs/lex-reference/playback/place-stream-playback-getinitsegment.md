---
title: place.stream.playback.getInitSegment
description: Reference for the place.stream.playback.getInitSegment lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get the HLS CMAF init segment (ftyp+moov) for a specific track of a video. Returns a single-track init segment suitable for use as an EXT-X-MAP in HLS media playlists.

**Parameters:**

| Name    | Type     | Req'd | Description                                  | Constraints   |
| ------- | -------- | ----- | -------------------------------------------- | ------------- |
| `did`   | `string` | ✅    | DID of the video creator.                    | Format: `did` |
| `rkey`  | `string` | ✅    | Record key of the place.stream.video record. |               |
| `track` | `string` | ✅    | Track ID to get the init segment for.        |               |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `VideoNotFound`: No video record found for this DID and rkey.
- `TrackNotFound`: No track with this ID found in the video.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getInitSegment",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get the HLS CMAF init segment (ftyp+moov) for a specific track of a video. Returns a single-track init segment suitable for use as an EXT-X-MAP in HLS media playlists.",
      "parameters": {
        "type": "params",
        "required": ["did", "rkey", "track"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "DID of the video creator."
          },
          "rkey": {
            "type": "string",
            "description": "Record key of the place.stream.video record."
          },
          "track": {
            "type": "string",
            "description": "Track ID to get the init segment for."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "VideoNotFound",
          "description": "No video record found for this DID and rkey."
        },
        {
          "name": "TrackNotFound",
          "description": "No track with this ID found in the video."
        }
      ]
    }
  }
}
```
