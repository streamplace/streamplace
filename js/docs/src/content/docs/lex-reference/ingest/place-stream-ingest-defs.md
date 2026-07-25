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
    }
  }
}
```
