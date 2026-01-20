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

| Name          | Type                                                                                        | Req'd | Description | Constraints      |
| ------------- | ------------------------------------------------------------------------------------------- | ----- | ----------- | ---------------- |
| `uri`         | `string`                                                                                    | ✅    |             | Format: `at-uri` |
| `cid`         | `string`                                                                                    | ✅    |             | Format: `cid`    |
| `record`      | `unknown`                                                                                   | ✅    |             |                  |
| `latestEvent` | [`place.stream.multistream.defs#event`](/lex-reference/place-stream-multistream-defs#event) | ❌    |             |                  |

---

<a name="event"></a>

### `event`

**Type:** `object`

**Properties:**

| Name        | Type     | Req'd | Description | Constraints                                    |
| ----------- | -------- | ----- | ----------- | ---------------------------------------------- |
| `message`   | `string` | ✅    |             |                                                |
| `status`    | `string` | ✅    |             | Enum: `inactive`, `pending`, `active`, `error` |
| `createdAt` | `string` | ✅    |             | Format: `datetime`                             |

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
        },
        "latestEvent": {
          "type": "ref",
          "ref": "place.stream.multistream.defs#event"
        }
      }
    },
    "event": {
      "type": "object",
      "required": ["message", "status", "createdAt"],
      "properties": {
        "message": {
          "type": "string"
        },
        "status": {
          "type": "string",
          "enum": ["inactive", "pending", "active", "error"]
        },
        "createdAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    }
  }
}
```
