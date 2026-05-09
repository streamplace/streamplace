---
title: place.stream.video
description: Reference for the place.stream.video lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Some media content: potentially video, audio, or subtitles.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                                                                                                                                                              | Req'd | Description                                                                                | Constraints                                   |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `createdAt` | `string`                                                                                                                                                                                                                          | ✅    | Time this video was created.                                                               | Format: `datetime`                            |
| `title`     | `string`                                                                                                                                                                                                                          | ✅    |                                                                                            | Max Length: 1400<br/>Max Graphemes: 140       |
| `duration`  | `integer`                                                                                                                                                                                                                         | ❌    | Total duration of the video in milliseconds.                                               |                                               |
| `source`    | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceTracks`](/lex-reference/place-stream-media-defs#sourcetracks)<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceClip`](/lex-reference/place-stream-media-defs#sourceclip) | ✅    | The canonical source of this video, either some media tracks or a clip from another video. |                                               |
| `thumb`     | `blob`                                                                                                                                                                                                                            | ❌    | Thumbnail image for the video.                                                             | Accept: `image/*`<br/>Max Size: 1000000 bytes |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.video",
  "defs": {
    "main": {
      "type": "record",
      "description": "Some media content: potentially video, audio, or subtitles.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["createdAt", "title", "source"],
        "properties": {
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Time this video was created."
          },
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140
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
          },
          "thumb": {
            "description": "Thumbnail image for the video.",
            "type": "blob",
            "accept": ["image/*"],
            "maxSize": 1000000
          }
        }
      }
    }
  }
}
```
