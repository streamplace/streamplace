---
title: place.stream.playback.getVideoPlaylist
description: Reference for the place.stream.playback.getVideoPlaylist lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an HLS CMAF playlist for a video. Returns a master playlist if no track is specified, or a media playlist for the given track. Supports arbitrary time range queries for DVR and VOD.

**Parameters:**

| Name    | Type      | Req'd | Description                                                                           | Constraints   |
| ------- | --------- | ----- | ------------------------------------------------------------------------------------- | ------------- |
| `did`   | `string`  | ✅    | DID of the video creator.                                                             | Format: `did` |
| `rkey`  | `string`  | ✅    | Record key of the place.stream.video record.                                          |               |
| `track` | `string`  | ❌    | Track ID for a media playlist. Omit for the master playlist.                          |               |
| `start` | `integer` | ❌    | Start time in milliseconds from the beginning of the video. Defaults to 0.            |               |
| `end`   | `integer` | ❌    | End time in milliseconds. Omit for a live EVENT playlist that extends to the present. |               |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `VideoNotFound`: No video record found for this DID and rkey.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getVideoPlaylist",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an HLS CMAF playlist for a video. Returns a master playlist if no track is specified, or a media playlist for the given track. Supports arbitrary time range queries for DVR and VOD.",
      "parameters": {
        "type": "params",
        "required": ["did", "rkey"],
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
            "description": "Track ID for a media playlist. Omit for the master playlist."
          },
          "start": {
            "type": "integer",
            "description": "Start time in milliseconds from the beginning of the video. Defaults to 0."
          },
          "end": {
            "type": "integer",
            "description": "End time in milliseconds. Omit for a live EVENT playlist that extends to the present."
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
        }
      ]
    }
  }
}
```
