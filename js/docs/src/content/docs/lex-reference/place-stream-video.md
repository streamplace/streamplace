---
title: place.stream.video
description: Reference for the place.stream.video lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Some audiovisual content. Continuously updated during a live stream; becomes a static VOD when the stream ends.

**Record Key:** `tid`

**Record Properties:**

| Name         | Type                                                                                                                                   | Req'd | Description                                                                                        | Constraints                             |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `creator`    | `string`                                                                                                                               | ✅    | The DID of the creator of this video.                                                              | Format: `did`                           |
| `createdAt`  | `string`                                                                                                                               | ✅    | When this video started recording.                                                                 | Format: `datetime`                      |
| `endedAt`    | `string`                                                                                                                               | ❌    | When recording ended. Absent while live.                                                           | Format: `datetime`                      |
| `title`      | `string`                                                                                                                               | ❌    |                                                                                                    | Max Length: 1400<br/>Max Graphemes: 140 |
| `catalog`    | [`#catalog`](#catalog)                                                                                                                 | ✅    | Track configuration metadata (codecs, resolution, channels, timescales).                           |                                         |
| `archive`    | `blob`                                                                                                                                 | ✅    | Blob ref to the MUXL archive fMP4. CID is the BLAKE-3 hash of the virtual archive file.            | Accept: `video/mp4`                     |
| `duration`   | `integer`                                                                                                                              | ❌    | Total duration of the video in nanoseconds. Updated as the stream grows.                           |                                         |
| `livestream` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | Back-reference to the place.stream.livestream record, if this video originated from a live stream. |                                         |

---

<a name="catalog"></a>

### `catalog`

**Type:** `object`

Track configuration describing all media tracks in the video.

**Properties:**

| Name    | Type                                  | Req'd | Description | Constraints |
| ------- | ------------------------------------- | ----- | ----------- | ----------- |
| `video` | Array of [`#videoTrack`](#videotrack) | ❌    |             |             |
| `audio` | Array of [`#audioTrack`](#audiotrack) | ❌    |             |             |

---

<a name="videotrack"></a>

### `videoTrack`

**Type:** `object`

**Properties:**

| Name        | Type      | Req'd | Description                                 | Constraints |
| ----------- | --------- | ----- | ------------------------------------------- | ----------- |
| `codec`     | `string`  | ✅    | WebCodecs codec string, e.g. 'avc1.64002A'. |             |
| `width`     | `integer` | ✅    | Coded pixel width.                          |             |
| `height`    | `integer` | ✅    | Coded pixel height.                         |             |
| `trackId`   | `integer` | ✅    | MP4 track ID.                               |             |
| `timescale` | `integer` | ✅    | Media timescale (ticks per second).         |             |

---

<a name="audiotrack"></a>

### `audioTrack`

**Type:** `object`

**Properties:**

| Name         | Type      | Req'd | Description                                       | Constraints |
| ------------ | --------- | ----- | ------------------------------------------------- | ----------- |
| `codec`      | `string`  | ✅    | WebCodecs codec string, e.g. 'opus', 'mp4a.40.2'. |             |
| `channels`   | `integer` | ✅    | Number of audio channels.                         |             |
| `sampleRate` | `integer` | ✅    | Sample rate in Hz.                                |             |
| `trackId`    | `integer` | ✅    | MP4 track ID.                                     |             |
| `timescale`  | `integer` | ✅    | Media timescale (ticks per second).               |             |

---

<a name="videoview"></a>

### `videoView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |             | Format: `cid`      |
| `creator`   | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.video",
  "defs": {
    "main": {
      "type": "record",
      "description": "Some audiovisual content. Continuously updated during a live stream; becomes a static VOD when the stream ends.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["creator", "createdAt", "catalog", "archive"],
        "properties": {
          "creator": {
            "type": "string",
            "format": "did",
            "description": "The DID of the creator of this video."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this video started recording."
          },
          "endedAt": {
            "type": "string",
            "format": "datetime",
            "description": "When recording ended. Absent while live."
          },
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140
          },
          "catalog": {
            "type": "ref",
            "ref": "#catalog",
            "description": "Track configuration metadata (codecs, resolution, channels, timescales)."
          },
          "archive": {
            "type": "blob",
            "accept": ["video/mp4"],
            "description": "Blob ref to the MUXL archive fMP4. CID is the BLAKE-3 hash of the virtual archive file."
          },
          "duration": {
            "type": "integer",
            "description": "Total duration of the video in nanoseconds. Updated as the stream grows."
          },
          "livestream": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "Back-reference to the place.stream.livestream record, if this video originated from a live stream."
          }
        }
      }
    },
    "catalog": {
      "type": "object",
      "description": "Track configuration describing all media tracks in the video.",
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
      "required": ["codec", "width", "height", "trackId", "timescale"],
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
      "required": ["codec", "channels", "sampleRate", "trackId", "timescale"],
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
    },
    "videoView": {
      "type": "object",
      "required": ["uri", "cid", "creator", "record", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "creator": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "record": {
          "type": "unknown"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    }
  }
}
```
