---
title: place.stream.video
description: Reference for the place.stream.video lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Some media content: potentially video, audio, or subtitles. This record is intentionally minimal so that metadata can be altered without breaking strongRefs. For the stream's title and description and whatnot, check the `place.stream.metadata.video` with the tid matching this video.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                                                                                                                                                              | Req'd | Description                                                                                | Constraints        |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------ | ------------------ |
| `createdAt` | `string`                                                                                                                                                                                                                          | ✅    | Time this video was created.                                                               | Format: `datetime` |
| `duration`  | `integer`                                                                                                                                                                                                                         | ✅    | Total duration of the video in milliseconds.                                               |                    |
| `source`    | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceTracks`](/lex-reference/place-stream-media-defs#sourcetracks)<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceClip`](/lex-reference/place-stream-media-defs#sourceclip) | ✅    | The canonical source of this video, either some media tracks or a clip from another video. |                    |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.video",
  "defs": {
    "main": {
      "type": "record",
      "description": "Some media content: potentially video, audio, or subtitles. This record is intentionally minimal so that metadata can be altered without breaking strongRefs. For the stream's title and description and whatnot, check the `place.stream.metadata.video` with the tid matching this video.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["createdAt", "source", "duration"],
        "properties": {
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Time this video was created."
          },
          "duration": {
            "type": "integer",
            "description": "Total duration of the video in milliseconds."
          },
          "source": {
            "type": "union",
            "refs": [
              "place.stream.media.defs#sourceTracks",
              "place.stream.media.defs#sourceClip"
            ],
            "description": "The canonical source of this video, either some media tracks or a clip from another video."
          }
        }
      }
    }
  }
}
```
