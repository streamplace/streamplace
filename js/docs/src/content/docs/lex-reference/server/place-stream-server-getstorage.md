---
title: place.stream.server.getStorage
description: Reference for the place.stream.server.getStorage lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get S3 storage configuration (with masked secret key).

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                  | Req'd | Description | Constraints |
| --------- | ------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `storage` | [`place.stream.server.defs#storage`](/lex-reference/place-stream-server-defs#storage) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.getStorage",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get S3 storage configuration (with masked secret key).",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {
            "storage": {
              "type": "ref",
              "ref": "place.stream.server.defs#storage"
            }
          }
        }
      }
    }
  }
}
```
