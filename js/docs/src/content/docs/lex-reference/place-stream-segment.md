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

| Name       | Type      | Req'd | Description | Constraints  |
| ---------- | --------- | ----- | ----------- | ------------ |
| `codec`    | `string`  | ✅    |             | Enum: `opus` |
| `rate`     | `integer` | ✅    |             |              |
| `channels` | `integer` | ✅    |             |              |

---

<a name="video"></a>

### `video`

**Type:** `object`

**Properties:**

| Name        | Type                       | Req'd | Description | Constraints  |
| ----------- | -------------------------- | ----- | ----------- | ------------ |
| `codec`     | `string`                   | ✅    |             | Enum: `h264` |
| `width`     | `integer`                  | ✅    |             |              |
| `height`    | `integer`                  | ✅    |             |              |
| `framerate` | [`#framerate`](#framerate) | ❌    |             |              |
| `bframes`   | `boolean`                  | ❌    |             |              |

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
          "enum": ["opus"]
        },
        "rate": {
          "type": "integer"
        },
        "channels": {
          "type": "integer"
        }
      }
    },
    "video": {
      "type": "object",
      "required": ["codec", "width", "height"],
      "properties": {
        "codec": {
          "type": "string",
          "enum": ["h264"]
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
