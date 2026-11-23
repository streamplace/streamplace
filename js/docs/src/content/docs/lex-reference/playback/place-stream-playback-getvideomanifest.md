---
title: place.stream.playback.getVideoManifest
description: Reference for the place.stream.playback.getVideoManifest lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a session-bound VOD manifest with embedded track initialization segments and ordered playback sources.

**Parameters:**

| Name    | Type      | Req'd | Description                               | Constraints      |
| ------- | --------- | ----- | ----------------------------------------- | ---------------- |
| `uri`   | `string`  | ✅    |                                           | Format: `at-uri` |
| `start` | `integer` | ❌    | Clip-local start in milliseconds.         |                  |
| `end`   | `integer` | ❌    | Clip-local exclusive end in milliseconds. |                  |
| `sid`   | `string`  | ❌    |                                           |                  |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type                          | Req'd | Description | Constraints      |
| ------------ | ----------------------------- | ----- | ----------- | ---------------- |
| `uri`        | `string`                      | ✅    |             | Format: `at-uri` |
| `sid`        | `string`                      | ✅    |             |                  |
| `durationMs` | `integer`                     | ✅    |             | Min: 0           |
| `tracks`     | Array of [`#track`](#track)   | ✅    |             |                  |
| `sources`    | Array of [`#source`](#source) | ✅    |             |                  |

---

<a name="track"></a>

### `track`

**Type:** `object`

**Properties:**

| Name         | Type               | Req'd | Description                    | Constraints                    |
| ------------ | ------------------ | ----- | ------------------------------ | ------------------------------ |
| `id`         | `string`           | ✅    |                                |                                |
| `type`       | `string`           | ✅    |                                | Known Values: `video`, `audio` |
| `codec`      | `string`           | ✅    |                                |                                |
| `width`      | `integer`          | ❌    |                                |                                |
| `height`     | `integer`          | ❌    |                                |                                |
| `channels`   | `integer`          | ❌    |                                |                                |
| `sampleRate` | `integer`          | ❌    |                                |                                |
| `bandwidth`  | `integer`          | ✅    |                                |                                |
| `timescale`  | `integer`          | ✅    |                                |                                |
| `init`       | `string`           | ✅    | Base64 initialization segment. |                                |
| `url`        | `string`           | ✅    |                                |                                |
| `first`      | `integer`          | ✅    |                                | Min: 0                         |
| `offsets`    | Array of `integer` | ✅    |                                |                                |
| `sizes`      | Array of `integer` | ✅    |                                |                                |
| `durations`  | Array of `integer` | ✅    |                                |                                |

---

<a name="source"></a>

### `source`

**Type:** `object`

**Properties:**

| Name          | Type     | Req'd | Description | Constraints                 |
| ------------- | -------- | ----- | ----------- | --------------------------- |
| `type`        | `string` | ✅    |             | Known Values: `moq`, `http` |
| `url`         | `string` | ❌    |             |                             |
| `fingerprint` | `string` | ❌    |             |                             |
| `broadcast`   | `string` | ❌    |             |                             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getVideoManifest",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a session-bound VOD manifest with embedded track initialization segments and ordered playback sources.",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "format": "at-uri"
          },
          "start": {
            "type": "integer",
            "description": "Clip-local start in milliseconds."
          },
          "end": {
            "type": "integer",
            "description": "Clip-local exclusive end in milliseconds."
          },
          "sid": {
            "type": "string"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "sid", "durationMs", "tracks", "sources"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri"
            },
            "sid": {
              "type": "string"
            },
            "durationMs": {
              "type": "integer",
              "minimum": 0
            },
            "tracks": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#track"
              }
            },
            "sources": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#source"
              }
            }
          }
        }
      }
    },
    "track": {
      "type": "object",
      "required": [
        "id",
        "type",
        "codec",
        "bandwidth",
        "timescale",
        "init",
        "url",
        "first",
        "offsets",
        "sizes",
        "durations"
      ],
      "properties": {
        "id": {
          "type": "string"
        },
        "type": {
          "type": "string",
          "knownValues": ["video", "audio"]
        },
        "codec": {
          "type": "string"
        },
        "width": {
          "type": "integer"
        },
        "height": {
          "type": "integer"
        },
        "channels": {
          "type": "integer"
        },
        "sampleRate": {
          "type": "integer"
        },
        "bandwidth": {
          "type": "integer"
        },
        "timescale": {
          "type": "integer"
        },
        "init": {
          "type": "string",
          "description": "Base64 initialization segment."
        },
        "url": {
          "type": "string"
        },
        "first": {
          "type": "integer",
          "minimum": 0
        },
        "offsets": {
          "type": "array",
          "items": {
            "type": "integer"
          }
        },
        "sizes": {
          "type": "array",
          "items": {
            "type": "integer"
          }
        },
        "durations": {
          "type": "array",
          "items": {
            "type": "integer"
          }
        }
      }
    },
    "source": {
      "type": "object",
      "required": ["type"],
      "properties": {
        "type": {
          "type": "string",
          "knownValues": ["moq", "http"]
        },
        "url": {
          "type": "string"
        },
        "fingerprint": {
          "type": "string"
        },
        "broadcast": {
          "type": "string"
        }
      }
    }
  }
}
```
