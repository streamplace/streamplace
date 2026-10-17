---
title: place.stream.ingest.defs
description: Reference for the place.stream.ingest.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="ingest"></a>

### `ingest`

**Type:** `object`

An ingest URL for a Streamplace station.

**Properties:**

| Name   | Type     | Req'd | Description                                                             | Constraints   |
| ------ | -------- | ----- | ----------------------------------------------------------------------- | ------------- |
| `type` | `string` | ✅    | The type of ingest endpoint, currently 'rtmp' and 'whip' are supported. |               |
| `url`  | `string` | ✅    | The URL of the ingest endpoint.                                         | Format: `uri` |

---

<a name="problems"></a>

### `problems`

**Type:** `object`

The problems a node currently sees with how a streamer is connected to it, sent over the livestream websocket whenever they change. An empty list clears earlier ones.

**Properties:**

| Name       | Type                            | Req'd | Description | Constraints |
| ---------- | ------------------------------- | ----- | ----------- | ----------- |
| `problems` | Array of [`#problem`](#problem) | ✅    |             |             |

---

<a name="problem"></a>

### `problem`

**Type:** `object`

Something the streamer should fix about how they're connected.

**Properties:**

| Name       | Type     | Req'd | Description                                       | Constraints                              |
| ---------- | -------- | ----- | ------------------------------------------------- | ---------------------------------------- |
| `code`     | `string` | ✅    | Stable identifier for the kind of problem.        | Known Values: `deprecated_ingest_host`   |
| `message`  | `string` | ✅    | What's wrong and how to fix it, for the streamer. |                                          |
| `severity` | `string` | ✅    |                                                   | Known Values: `error`, `warning`, `info` |
| `link`     | `string` | ❌    | Where to read more.                               | Format: `uri`                            |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.ingest.defs",
  "defs": {
    "ingest": {
      "type": "object",
      "description": "An ingest URL for a Streamplace station.",
      "required": ["type", "url"],
      "properties": {
        "type": {
          "type": "string",
          "description": "The type of ingest endpoint, currently 'rtmp' and 'whip' are supported."
        },
        "url": {
          "type": "string",
          "format": "uri",
          "description": "The URL of the ingest endpoint."
        }
      }
    },
    "problems": {
      "type": "object",
      "description": "The problems a node currently sees with how a streamer is connected to it, sent over the livestream websocket whenever they change. An empty list clears earlier ones.",
      "required": ["problems"],
      "properties": {
        "problems": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#problem"
          }
        }
      }
    },
    "problem": {
      "type": "object",
      "description": "Something the streamer should fix about how they're connected.",
      "required": ["code", "message", "severity"],
      "properties": {
        "code": {
          "type": "string",
          "description": "Stable identifier for the kind of problem.",
          "knownValues": ["deprecated_ingest_host"]
        },
        "message": {
          "type": "string",
          "description": "What's wrong and how to fix it, for the streamer."
        },
        "severity": {
          "type": "string",
          "knownValues": ["error", "warning", "info"]
        },
        "link": {
          "type": "string",
          "format": "uri",
          "description": "Where to read more."
        }
      }
    }
  }
}
```
