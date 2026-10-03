---
title: place.stream.caption.getCaptions
description: Reference for the place.stream.caption.getCaptions lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Download one caption track as WebVTT, SRT, or JSON cues. For live streams this returns the captions in the node's current window, with times counted from the window's epoch (the start of the first segment the node holds; JSON carries it as `epoch`). For videos times count from the video start. `start`, `end`, and `mpegts` make a response usable as an HLS WebVTT subtitle segment.

**Parameters:**

| Name       | Type      | Req'd | Description                                                                                                                                                                     | Constraints                                           |
| ---------- | --------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------- |
| `streamer` | `string`  | ❌    | DID or handle of a live streamer. Mutually exclusive with video.                                                                                                                |                                                       |
| `video`    | `string`  | ❌    |                                                                                                                                                                                 | Format: `at-uri`                                      |
| `track`    | `string`  | ✅    | Track id from listTracks.                                                                                                                                                       |                                                       |
| `format`   | `string`  | ❌    |                                                                                                                                                                                 | Known Values: `vtt`, `srt`, `json`<br/>Default: `vtt` |
| `start`    | `integer` | ❌    | Only cues overlapping [start, end), in milliseconds from the video start. Videos only.                                                                                          |                                                       |
| `end`      | `integer` | ❌    | End of the cue range in milliseconds. Videos only.                                                                                                                              |                                                       |
| `mpegts`   | `integer` | ❌    | Adds an X-TIMESTAMP-MAP header (MPEGTS:<value>,LOCAL:00:00:00.000) to a WebVTT response: the 90kHz media timestamp that cue time zero plays at. Used by HLS subtitle playlists. |                                                       |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `NotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.caption.getCaptions",
  "defs": {
    "main": {
      "type": "query",
      "description": "Download one caption track as WebVTT, SRT, or JSON cues. For live streams this returns the captions in the node's current window, with times counted from the window's epoch (the start of the first segment the node holds; JSON carries it as `epoch`). For videos times count from the video start. `start`, `end`, and `mpegts` make a response usable as an HLS WebVTT subtitle segment.",
      "parameters": {
        "type": "params",
        "required": ["track"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "DID or handle of a live streamer. Mutually exclusive with video."
          },
          "video": {
            "type": "string",
            "format": "at-uri"
          },
          "track": {
            "type": "string",
            "description": "Track id from listTracks."
          },
          "format": {
            "type": "string",
            "knownValues": ["vtt", "srt", "json"],
            "default": "vtt"
          },
          "start": {
            "type": "integer",
            "description": "Only cues overlapping [start, end), in milliseconds from the video start. Videos only."
          },
          "end": {
            "type": "integer",
            "description": "End of the cue range in milliseconds. Videos only."
          },
          "mpegts": {
            "type": "integer",
            "description": "Adds an X-TIMESTAMP-MAP header (MPEGTS:<value>,LOCAL:00:00:00.000) to a WebVTT response: the 90kHz media timestamp that cue time zero plays at. Used by HLS subtitle playlists."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "NotFound"
        }
      ]
    }
  }
}
```
