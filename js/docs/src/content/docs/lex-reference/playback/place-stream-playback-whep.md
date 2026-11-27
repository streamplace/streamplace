---
title: place.stream.playback.whep
description: Reference for the place.stream.playback.whep lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Play a stream over WebRTC using WHEP.

**Parameters:**

| Name        | Type     | Req'd | Description                                                                                  | Constraints |
| ----------- | -------- | ----- | -------------------------------------------------------------------------------------------- | ----------- |
| `streamer`  | `string` | ✅    | The DID of the streamer to play.                                                             |             |
| `rendition` | `string` | ✅    | The rendition of the stream to play.                                                         |             |
| `sid`       | `string` | ❌    | The streamer's own playback session (getPlaybackSession), which opens their pre-live stream. |             |

**Input:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `Unauthorized`: This user may not play this stream.
- `PlaybackStartedElsewhere`: A pre-live stream plays to one owner session at a time, and a newer one has taken over; mint a new session to play here again.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.whep",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Play a stream over WebRTC using WHEP.",
      "parameters": {
        "type": "params",
        "required": ["streamer", "rendition"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "The DID of the streamer to play."
          },
          "rendition": {
            "type": "string",
            "description": "The rendition of the stream to play."
          },
          "sid": {
            "type": "string",
            "description": "The streamer's own playback session (getPlaybackSession), which opens their pre-live stream."
          }
        }
      },
      "input": {
        "encoding": "*/*"
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "This user may not play this stream."
        },
        {
          "name": "PlaybackStartedElsewhere",
          "description": "A pre-live stream plays to one owner session at a time, and a newer one has taken over; mint a new session to play here again."
        }
      ]
    }
  }
}
```
