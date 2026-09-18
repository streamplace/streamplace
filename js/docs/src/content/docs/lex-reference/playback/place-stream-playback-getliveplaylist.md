---
title: place.stream.playback.getLivePlaylist
description: Reference for the place.stream.playback.getLivePlaylist lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an HLS CMAF playlist for a live stream. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getLiveSegment. Segments come from an in-memory sliding window fed as the stream is ingested (or replicated to this node), so a playlist is only available while the stream is live here.

**Parameters:**

| Name       | Type     | Req'd | Description                                                                                                                                                                                      | Constraints |
| ---------- | -------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| `streamer` | `string` | ✅    | The streamer to play back: a DID (did:plc/did:web/did:key) or a Bluesky handle, which is resolved to its DID.                                                                                    |             |
| `track`    | `string` | ❌    | Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist.                                                                          |             |
| `sid`      | `string` | ❌    | Opaque playback session identifier. Omit on the master playlist request; the server generates one and threads it through the sub-playlist + segment URLs it returns, for view-count correlation. |             |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `StreamNotLive`: No live segments are currently windowed for this streamer on this node.
- `TrackNotFound`: The requested track ID is not present in the live stream.
- `StreamUnavailable`: The streamer's account is unavailable (e.g. banned).

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getLivePlaylist",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an HLS CMAF playlist for a live stream. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getLiveSegment. Segments come from an in-memory sliding window fed as the stream is ingested (or replicated to this node), so a playlist is only available while the stream is live here.",
      "parameters": {
        "type": "params",
        "required": ["streamer"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "The streamer to play back: a DID (did:plc/did:web/did:key) or a Bluesky handle, which is resolved to its DID."
          },
          "track": {
            "type": "string",
            "description": "Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist."
          },
          "sid": {
            "type": "string",
            "description": "Opaque playback session identifier. Omit on the master playlist request; the server generates one and threads it through the sub-playlist + segment URLs it returns, for view-count correlation."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "StreamNotLive",
          "description": "No live segments are currently windowed for this streamer on this node."
        },
        {
          "name": "TrackNotFound",
          "description": "The requested track ID is not present in the live stream."
        },
        {
          "name": "StreamUnavailable",
          "description": "The streamer's account is unavailable (e.g. banned)."
        }
      ]
    }
  }
}
```
