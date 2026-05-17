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

| Name     | Type                                                                                                                                            | Req'd | Description                                                    | Constraints |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------- | ----------- |
| `tracks` | Array of [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | The canonical list of tracks specifying the source of a video. |             |

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

| Name         | Type      | Req'd | Description                                                                                                                                                                 | Constraints   |
| ------------ | --------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `blob`       | `string`  | ✅    | BLAKE-3 content hash (BDASL CID) of the source video segment.                                                                                                               |               |
| `size`       | `integer` | ❌    | Size in bytes of the source video segment.                                                                                                                                  |               |
| `trackId`    | `string`  | ✅    | ID of the track within the MUXL container. 1-indexed for MP4 reasons.                                                                                                       |               |
| `mediaType`  | `string`  | ✅    | Type of the track: video, audio, or text.                                                                                                                                   |               |
| `language`   | `string`  | ❌    | Language of the track, if applicable.                                                                                                                                       |               |
| `signingKey` | `string`  | ❌    | did:key of the ephemeral key that C2PA-signed this track's segments. The private key is discarded once the track is produced, so it can only ever have signed this content. | Format: `did` |

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
            "type": "ref",
            "ref": "com.atproto.repo.strongRef"
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
      "required": ["blob", "trackId", "mediaType"],
      "properties": {
        "blob": {
          "type": "string",
          "description": "BLAKE-3 content hash (BDASL CID) of the source video segment."
        },
        "size": {
          "type": "integer",
          "description": "Size in bytes of the source video segment."
        },
        "trackId": {
          "type": "string",
          "description": "ID of the track within the MUXL container. 1-indexed for MP4 reasons."
        },
        "mediaType": {
          "type": "string",
          "description": "Type of the track: video, audio, or text."
        },
        "language": {
          "type": "string",
          "description": "Language of the track, if applicable."
        },
        "signingKey": {
          "type": "string",
          "format": "did",
          "description": "did:key of the ephemeral key that C2PA-signed this track's segments. The private key is discarded once the track is produced, so it can only ever have signed this content."
        }
      }
    }
  }
}
```
