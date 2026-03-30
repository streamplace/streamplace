---
title: place.stream.muxl.defs
description: Reference for the place.stream.muxl.defs lexicon
---
**Lexicon Version:** 1

## Definitions

<a name="archive"></a>
### `archive`

**Type:** `object`

A MUXL archive fMP4 file, identified by its BLAKE-3 content hash.

**Properties:**

| Name | Type | Req'd  | Description | Constraints |
|------|------|----------|-------------|-------------|
| `data` | `blob` | ✅  | Blob ref to the MUXL archive fMP4. The CID is the BLAKE-3 hash of the file. | Accept: `video/mp4` |

---

<a name="catalog"></a>
### `catalog`

**Type:** `object`

Track configuration describing all media tracks.

**Properties:**

| Name | Type | Req'd  | Description | Constraints |
|------|------|----------|-------------|-------------|
| `video` | Array of [`#videoTrack`](#videotrack) | ❌  |  |  |
| `audio` | Array of [`#audioTrack`](#audiotrack) | ❌  |  |  |

---

<a name="videotrack"></a>
### `videoTrack`

**Type:** `object`

**Properties:**

| Name | Type | Req'd  | Description | Constraints |
|------|------|----------|-------------|-------------|
| `codec` | `string` | ✅  | WebCodecs codec string, e.g. 'avc1.64002A'. |  |
| `width` | `integer` | ✅  | Coded pixel width. |  |
| `height` | `integer` | ✅  | Coded pixel height. |  |
| `trackId` | `integer` | ✅  | MP4 track ID. |  |
| `timescale` | `integer` | ✅  | Media timescale (ticks per second). |  |

---

<a name="audiotrack"></a>
### `audioTrack`

**Type:** `object`

**Properties:**

| Name | Type | Req'd  | Description | Constraints |
|------|------|----------|-------------|-------------|
| `codec` | `string` | ✅  | WebCodecs codec string, e.g. 'opus', 'mp4a.40.2'. |  |
| `channels` | `integer` | ✅  | Number of audio channels. |  |
| `sampleRate` | `integer` | ✅  | Sample rate in Hz. |  |
| `trackId` | `integer` | ✅  | MP4 track ID. |  |
| `timescale` | `integer` | ✅  | Media timescale (ticks per second). |  |

---

## Lexicon Source
```json
{
  "lexicon": 1,
  "id": "place.stream.muxl.defs",
  "defs": {
    "archive": {
      "type": "object",
      "description": "A MUXL archive fMP4 file, identified by its BLAKE-3 content hash.",
      "required": [
        "data"
      ],
      "properties": {
        "data": {
          "type": "blob",
          "accept": [
            "video/mp4"
          ],
          "description": "Blob ref to the MUXL archive fMP4. The CID is the BLAKE-3 hash of the file."
        }
      }
    },
    "catalog": {
      "type": "object",
      "description": "Track configuration describing all media tracks.",
      "required": [],
      "properties": {
        "video": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#videoTrack"
          }
        },
        "audio": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#audioTrack"
          }
        }
      }
    },
    "videoTrack": {
      "type": "object",
      "required": [
        "codec",
        "width",
        "height",
        "trackId",
        "timescale"
      ],
      "properties": {
        "codec": {
          "type": "string",
          "description": "WebCodecs codec string, e.g. 'avc1.64002A'."
        },
        "width": {
          "type": "integer",
          "description": "Coded pixel width."
        },
        "height": {
          "type": "integer",
          "description": "Coded pixel height."
        },
        "trackId": {
          "type": "integer",
          "description": "MP4 track ID."
        },
        "timescale": {
          "type": "integer",
          "description": "Media timescale (ticks per second)."
        }
      }
    },
    "audioTrack": {
      "type": "object",
      "required": [
        "codec",
        "channels",
        "sampleRate",
        "trackId",
        "timescale"
      ],
      "properties": {
        "codec": {
          "type": "string",
          "description": "WebCodecs codec string, e.g. 'opus', 'mp4a.40.2'."
        },
        "channels": {
          "type": "integer",
          "description": "Number of audio channels."
        },
        "sampleRate": {
          "type": "integer",
          "description": "Sample rate in Hz."
        },
        "trackId": {
          "type": "integer",
          "description": "MP4 track ID."
        },
        "timescale": {
          "type": "integer",
          "description": "Media timescale (ticks per second)."
        }
      }
    }
  }
}
```
