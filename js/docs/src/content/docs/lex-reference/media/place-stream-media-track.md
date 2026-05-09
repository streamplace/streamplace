---
title: place.stream.media.track
description: Reference for the place.stream.media.track lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

An additional track for a video stream.

**Record Key:** `tid`

**Record Properties:**

| Name    | Type                                                                                                              | Req'd | Description                                   | Constraints      |
| ------- | ----------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------- | ---------------- |
| `video` | `string`                                                                                                          | ✅    | The video that this track is associated with. | Format: `at-uri` |
| `track` | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#muxlTrack`](/lex-reference/place-stream-media-defs#muxltrack) | ✅    |                                               |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.track",
  "defs": {
    "main": {
      "type": "record",
      "description": "An additional track for a video stream.",
      "key": "tid",
      "record": {
        "required": ["video", "track"],
        "type": "object",
        "properties": {
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "The video that this track is associated with."
          },
          "track": {
            "type": "union",
            "refs": ["place.stream.media.defs#muxlTrack"]
          }
        }
      }
    }
  }
}
```
