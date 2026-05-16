---
title: place.stream.playback.getVideoPlaylist
description: Reference for the place.stream.playback.getVideoPlaylist lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an HLS CMAF playlist for a video. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getVideoBlob, addressed by content hash. The `uri` is an AT-URI pointing at a playable record — today only place.stream.video records are supported, but the surface is collection-agnostic so other record types can join later.

**Parameters:**

| Name    | Type      | Req'd | Description                                                                                                                                                                                                                                                                                                                                                                                                               | Constraints      |
| ------- | --------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------- |
| `uri`   | `string`  | ✅    | AT-URI of the record to play back (e.g. at://did:plc:.../place.stream.video/<rkey>).                                                                                                                                                                                                                                                                                                                                      | Format: `at-uri` |
| `track` | `string`  | ❌    | Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist.                                                                                                                                                                                                                                                                                                   |                  |
| `start` | `integer` | ❌    | Start time in milliseconds from the beginning of the video. Defaults to 0. For a place.stream.video record whose source is a place.stream.media.defs#sourceClip, this is in the clip's local timeline (0 == the clip's start).                                                                                                                                                                                            |                  |
| `end`   | `integer` | ❌    | End time in milliseconds. Omit to include all remaining content. Local to the clip's timeline when playing back a sourceClip record.                                                                                                                                                                                                                                                                                      |                  |
| `sid`   | `string`  | ❌    | Opaque playback session identifier. Omit on the master playlist request; the server generates one and embeds it in every sub-playlist URL it returns. Players never have to construct it themselves — they just follow the URLs the master playlist hands them, which carry the sid into media-playlist + segment requests. Used downstream to correlate a player's playlist + segment fetches for view-count accounting. |                  |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `VideoNotFound`: No record indexed at the supplied AT-URI.
- `TrackNotFound`: The requested track ID is not present in the video's blob.
- `BlobNotFound`: The record references a blob this node hasn't indexed an origin for.
- `UnsupportedCollection`: The AT-URI points at a collection this endpoint doesn't know how to play back.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getVideoPlaylist",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an HLS CMAF playlist for a video. Returns a master playlist when `track` is omitted, or a single-track media playlist when `track` is supplied. The playlist references each segment + per-track init segment via getVideoBlob, addressed by content hash. The `uri` is an AT-URI pointing at a playable record — today only place.stream.video records are supported, but the surface is collection-agnostic so other record types can join later.",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the record to play back (e.g. at://did:plc:.../place.stream.video/<rkey>)."
          },
          "track": {
            "type": "string",
            "description": "Track ID (stringified u32 matching the MUXL container) for a single-track media playlist. Omit for the master playlist."
          },
          "start": {
            "type": "integer",
            "description": "Start time in milliseconds from the beginning of the video. Defaults to 0. For a place.stream.video record whose source is a place.stream.media.defs#sourceClip, this is in the clip's local timeline (0 == the clip's start)."
          },
          "end": {
            "type": "integer",
            "description": "End time in milliseconds. Omit to include all remaining content. Local to the clip's timeline when playing back a sourceClip record."
          },
          "sid": {
            "type": "string",
            "description": "Opaque playback session identifier. Omit on the master playlist request; the server generates one and embeds it in every sub-playlist URL it returns. Players never have to construct it themselves — they just follow the URLs the master playlist hands them, which carry the sid into media-playlist + segment requests. Used downstream to correlate a player's playlist + segment fetches for view-count accounting."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "VideoNotFound",
          "description": "No record indexed at the supplied AT-URI."
        },
        {
          "name": "TrackNotFound",
          "description": "The requested track ID is not present in the video's blob."
        },
        {
          "name": "BlobNotFound",
          "description": "The record references a blob this node hasn't indexed an origin for."
        },
        {
          "name": "UnsupportedCollection",
          "description": "The AT-URI points at a collection this endpoint doesn't know how to play back."
        }
      ]
    }
  }
}
```
