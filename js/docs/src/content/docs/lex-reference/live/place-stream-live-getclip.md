---
title: place.stream.live.getClip
description: Reference for the place.stream.live.getClip lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Clip a livestream between two instants as a flat (faststart) MP4. The clip is cut on MUXL segment boundaries: it holds every segment whose start falls within [start, end], so it may begin up to one segment late and run up to one segment past end. Segments come from the node's livestream recordings and from the recent stream it holds in memory; each segment's C2PA signature passes through verbatim. Only published (live) segments are clipped. Requires a signed-in caller: an OAuth session, or a service-auth token bound to this method (lxm), so third-party apps can call it through the PDS with the rpc:place.stream.live.getClip scope.

**Parameters:**

| Name       | Type     | Req'd | Description                                                                     | Constraints        |
| ---------- | -------- | ----- | ------------------------------------------------------------------------------- | ------------------ |
| `streamer` | `string` | ✅    | The streamer: a DID or a Bluesky handle (resolved to its DID).                  |                    |
| `start`    | `string` | ✅    | Start of the clip (wall-clock time).                                            | Format: `datetime` |
| `end`      | `string` | ✅    | End of the clip (wall-clock time). Must be after start, at most one hour later. | Format: `datetime` |

**Output:**

- **Encoding:** `video/mp4`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `ClipNotFound`: This node has no published segments of the stream in the requested range.
- `ClipUnavailable`: Part of the requested range is still being recorded and is held only by the node the streamer is streaming to; retry there, or later once the recording has been written.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.getClip",
  "defs": {
    "main": {
      "type": "query",
      "description": "Clip a livestream between two instants as a flat (faststart) MP4. The clip is cut on MUXL segment boundaries: it holds every segment whose start falls within [start, end], so it may begin up to one segment late and run up to one segment past end. Segments come from the node's livestream recordings and from the recent stream it holds in memory; each segment's C2PA signature passes through verbatim. Only published (live) segments are clipped. Requires a signed-in caller: an OAuth session, or a service-auth token bound to this method (lxm), so third-party apps can call it through the PDS with the rpc:place.stream.live.getClip scope.",
      "parameters": {
        "type": "params",
        "required": ["streamer", "start", "end"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "The streamer: a DID or a Bluesky handle (resolved to its DID)."
          },
          "start": {
            "type": "string",
            "format": "datetime",
            "description": "Start of the clip (wall-clock time)."
          },
          "end": {
            "type": "string",
            "format": "datetime",
            "description": "End of the clip (wall-clock time). Must be after start, at most one hour later."
          }
        }
      },
      "output": {
        "encoding": "video/mp4"
      },
      "errors": [
        {
          "name": "ClipNotFound",
          "description": "This node has no published segments of the stream in the requested range."
        },
        {
          "name": "ClipUnavailable",
          "description": "Part of the requested range is still being recorded and is held only by the node the streamer is streaming to; retry there, or later once the recording has been written."
        }
      ]
    }
  }
}
```
