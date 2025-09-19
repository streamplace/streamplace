---
title: place.stream.multistream.defs
description: Reference for the place.stream.multistream.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="targetview"></a>

### `targetView`

**Type:** `object`

**Properties:**

| Name     | Type      | Req'd | Description | Constraints      |
| -------- | --------- | ----- | ----------- | ---------------- |
| `uri`    | `string`  | ✅    |             | Format: `at-uri` |
| `cid`    | `string`  | ✅    |             | Format: `cid`    |
| `record` | `unknown` | ✅    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.multistream.defs",
  "defs": {
    "targetView": {
      "type": "object",
      "required": ["uri", "cid", "record"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
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
