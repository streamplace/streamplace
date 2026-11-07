---
title: place.stream.segment
description: Reference for the place.stream.segment lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Media file representing a segment of a livestream

**Record Key:** `tid`

**Record Properties:**

| Name                 | Type                                                                                                  | Req'd | Description                                      | Constraints        |
| -------------------- | ----------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------ | ------------------ |
| `id`                 | `string`                                                                                              | ✅    | Unique identifier for the segment                |                    |
| `signingKey`         | `string`                                                                                              | ✅    | The DID of the signing key used for this segment |                    |
| `startTime`          | `string`                                                                                              | ✅    | When this segment started                        | Format: `datetime` |
| `duration`           | `integer`                                                                                             | ❌    | The duration of the segment in nanoseconds       |                    |
| `creator`            | `string`                                                                                              | ✅    |                                                  | Format: `did`      |
| `video`              | Array of [`#video`](#video)                                                                           | ❌    |                                                  |                    |
| `audio`              | Array of [`#audio`](#audio)                                                                           | ❌    |                                                  |                    |
| `size`               | `integer`                                                                                             | ❌    | The size of the segment in bytes                 |                    |
| `contentWarnings`    | [`place.stream.metadata.contentWarnings`](/lex-reference/place-stream-metadata-contentwarnings)       | ❌    |                                                  |                    |
| `contentRights`      | [`place.stream.metadata.contentRights`](/lex-reference/place-stream-metadata-contentrights)           | ❌    |                                                  |                    |
| `distributionPolicy` | [`place.stream.metadata.distributionPolicy`](/lex-reference/place-stream-metadata-distributionpolicy) | ❌    |                                                  |                    |

---

<a name="audio"></a>

### `audio`

**Type:** `object`

**Properties:**

| Name             | Type      | Req'd | Description                                                                                                                                                                                                                                                                               | Constraints                           |
| ---------------- | --------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `codec`          | `string`  | ✅    |                                                                                                                                                                                                                                                                                           | Enum: `opus`, `aac`                   |
| `rate`           | `integer` | ✅    |                                                                                                                                                                                                                                                                                           |                                       |
| `channels`       | `integer` | ✅    |                                                                                                                                                                                                                                                                                           |                                       |
| `alternateGroup` | `integer` | ❌    | Audio tracks sharing this value carry the same sound in different codecs. Tracks with different values are independent and play together: the lowest group is the full mix, and higher groups are stems a viewer can mix in. Absent when the stream doesn't distinguish its audio tracks. | Min: 1                                |
| `label`          | `string`  | ❌    | The streamer's name for this audio track, such as "Music".                                                                                                                                                                                                                                | Max Length: 640<br/>Max Graphemes: 64 |

---

<a name="video"></a>

### `video`

**Type:** `object`

**Properties:**

| Name        | Type                       | Req'd | Description | Constraints         |
| ----------- | -------------------------- | ----- | ----------- | ------------------- |
| `codec`     | `string`                   | ✅    |             | Enum: `h264`, `av1` |
| `width`     | `integer`                  | ✅    |             |                     |
| `height`    | `integer`                  | ✅    |             |                     |
| `framerate` | [`#framerate`](#framerate) | ❌    |             |                     |
| `bframes`   | `boolean`                  | ❌    |             |                     |

---

<a name="framerate"></a>

### `framerate`

**Type:** `object`

**Properties:**

| Name  | Type      | Req'd | Description | Constraints |
| ----- | --------- | ----- | ----------- | ----------- |
| `num` | `integer` | ✅    |             |             |
| `den` | `integer` | ✅    |             |             |

---

<a name="segmentview"></a>

### `segmentView`

**Type:** `object`

**Properties:**

| Name     | Type      | Req'd | Description | Constraints   |
| -------- | --------- | ----- | ----------- | ------------- |
| `cid`    | `string`  | ✅    |             | Format: `cid` |
| `record` | `unknown` | ✅    |             |               |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.segment",
  "defs": {
    "main": {
      "type": "record",
      "description": "Media file representing a segment of a livestream",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["id", "signingKey", "startTime", "creator"],
        "properties": {
          "id": {
            "type": "string",
            "description": "Unique identifier for the segment"
          },
          "signingKey": {
            "type": "string",
            "description": "The DID of the signing key used for this segment"
          },
          "startTime": {
            "type": "string",
            "format": "datetime",
            "description": "When this segment started"
          },
          "duration": {
            "type": "integer",
            "description": "The duration of the segment in nanoseconds"
          },
          "creator": {
            "type": "string",
            "format": "did"
          },
          "video": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#video"
            }
          },
          "audio": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#audio"
            }
          },
          "size": {
            "type": "integer",
            "description": "The size of the segment in bytes"
          },
          "contentWarnings": {
            "type": "ref",
            "ref": "place.stream.metadata.contentWarnings"
          },
          "contentRights": {
            "type": "ref",
            "ref": "place.stream.metadata.contentRights"
          },
          "distributionPolicy": {
            "type": "ref",
            "ref": "place.stream.metadata.distributionPolicy"
          }
        }
      }
    },
    "audio": {
      "type": "object",
      "required": ["codec", "rate", "channels"],
      "properties": {
        "codec": {
          "type": "string",
          "enum": ["opus", "aac"]
        },
        "rate": {
          "type": "integer"
        },
        "channels": {
          "type": "integer"
        },
        "alternateGroup": {
          "type": "integer",
          "minimum": 1,
          "description": "Audio tracks sharing this value carry the same sound in different codecs. Tracks with different values are independent and play together: the lowest group is the full mix, and higher groups are stems a viewer can mix in. Absent when the stream doesn't distinguish its audio tracks."
        },
        "label": {
          "type": "string",
          "maxLength": 640,
          "maxGraphemes": 64,
          "description": "The streamer's name for this audio track, such as \"Music\"."
        }
      }
    },
    "video": {
      "type": "object",
      "required": ["codec", "width", "height"],
      "properties": {
        "codec": {
          "type": "string",
          "enum": ["h264", "av1"]
        },
        "width": {
          "type": "integer"
        },
        "height": {
          "type": "integer"
        },
        "framerate": {
          "type": "ref",
          "ref": "#framerate"
        },
        "bframes": {
          "type": "boolean"
        }
      }
    },
    "framerate": {
      "type": "object",
      "required": ["num", "den"],
      "properties": {
        "num": {
          "type": "integer"
        },
        "den": {
          "type": "integer"
        }
      }
    },
    "segmentView": {
      "type": "object",
      "required": ["cid", "record"],
      "properties": {
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "record": {
          "type": "unknown"
        }
      }
    }
  }
}
```
