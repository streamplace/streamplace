---
title: place.stream.media.defs
description: Reference for the place.stream.media.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="sourcetracks"></a>

### `sourceTracks`

**Type:** `object`

A collection of tracks representing the canonical source of a video.

**Properties:**

| Name     | Type                                                                                                                       | Req'd | Description                                                    | Constraints |
| -------- | -------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------- | ----------- |
| `tracks` | Array of Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#muxlTrack`](/lex-reference/place-stream-media-defs#muxltrack) | ✅    | The canonical list of tracks specifying the source of a video. |             |

---

<a name="sourceclip"></a>

### `sourceClip`

**Type:** `object`

An object representing that this video's source is a clip from another video.

**Properties:**

| Name    | Type      | Req'd | Description                             | Constraints      |
| ------- | --------- | ----- | --------------------------------------- | ---------------- |
| `video` | `string`  | ✅    | AT URI of the video we're clipping.     | Format: `at-uri` |
| `start` | `integer` | ✅    | Start time of the clip in milliseconds. |                  |
| `end`   | `integer` | ✅    | End time of the clip in milliseconds.   |                  |

---

<a name="muxltrack"></a>

### `muxlTrack`

**Type:** `object`

A track backed by a MUXL container

**Properties:**

| Name      | Type                                                                          | Req'd | Description                                | Constraints |
| --------- | ----------------------------------------------------------------------------- | ----- | ------------------------------------------ | ----------- |
| `blob`    | [`place.stream.media.defs#blob`](/lex-reference/place-stream-media-defs#blob) | ✅    |                                            |             |
| `trackId` | `string`                                                                      | ✅    | ID of the track within the MUXL container. |             |
| `type`    | `string`                                                                      | ✅    | Type of the track: video, audio, or text.  |             |

---

<a name="blob"></a>

### `blob`

**Type:** `object`

A MUXL blob in one of the MUXL-supported formats.

**Properties:**

| Name       | Type      | Req'd | Description                                      | Constraints |
| ---------- | --------- | ----- | ------------------------------------------------ | ----------- |
| `ref`      | `string`  | ✅    | BLAKE-3 content hash (BDASL CID) of the archive. |             |
| `muxlType` | `string`  | ✅    | MUXL type of the archive (mp4, fmp4).            |             |
| `size`     | `integer` | ✅    | Size of the file in bytes.                       |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.defs",
  "defs": {
    "sourceTracks": {
      "type": "object",
      "description": "A collection of tracks representing the canonical source of a video.",
      "required": ["tracks"],
      "properties": {
        "tracks": {
          "type": "array",
          "description": "The canonical list of tracks specifying the source of a video.",
          "items": {
            "type": "union",
            "refs": ["place.stream.media.defs#muxlTrack"]
          }
        }
      }
    },
    "sourceClip": {
      "type": "object",
      "description": "An object representing that this video's source is a clip from another video.",
      "required": ["video", "start", "end"],
      "properties": {
        "video": {
          "type": "string",
          "format": "at-uri",
          "description": "AT URI of the video we're clipping."
        },
        "start": {
          "type": "integer",
          "description": "Start time of the clip in milliseconds."
        },
        "end": {
          "type": "integer",
          "description": "End time of the clip in milliseconds."
        }
      }
    },
    "muxlTrack": {
      "type": "object",
      "description": "A track backed by a MUXL container",
      "required": ["blob", "trackId", "type"],
      "properties": {
        "blob": {
          "type": "ref",
          "ref": "place.stream.media.defs#blob"
        },
        "trackId": {
          "type": "string",
          "description": "ID of the track within the MUXL container."
        },
        "type": {
          "type": "string",
          "description": "Type of the track: video, audio, or text."
        }
      }
    },
    "blob": {
      "type": "object",
      "description": "A MUXL blob in one of the MUXL-supported formats.",
      "required": ["ref", "muxlType", "size"],
      "properties": {
        "ref": {
          "type": "string",
          "description": "BLAKE-3 content hash (BDASL CID) of the archive."
        },
        "muxlType": {
          "type": "string",
          "description": "MUXL type of the archive (mp4, fmp4)."
        },
        "size": {
          "type": "integer",
          "description": "Size of the file in bytes."
        }
      }
    }
  }
}
```
